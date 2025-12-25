package main

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"io"
	"item-archive-d/internal/blob"
	"item-archive-d/internal/db"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"golang.org/x/time/rate"
	"google.golang.org/genai"
)

type Model struct {
	ID            string
	DayLimiter    *rate.Limiter
	MinuteLimiter *rate.Limiter
}

var (
	gemini_2_5_flash_free = Model{
		ID: "gemini-2.5-flash",
		// burst = max request count so that you are not waiting for an average
		// rate to make more requests when more requests can be made
		MinuteLimiter: rate.NewLimiter(rate.Every(time.Minute/5), 5),
		DayLimiter:    rate.NewLimiter(rate.Every(time.Hour*24/20), 20),
	}
	gemini_2_5_flash_lite_free = Model{
		ID:            "gemini-2.5-flash-lite",
		MinuteLimiter: rate.NewLimiter(rate.Every(time.Minute/10), 10),
		DayLimiter:    rate.NewLimiter(rate.Every(time.Hour*24/20), 20),
	}
	gemma_3_27b_free = Model{
		ID:            "gemma-3-27b-it",
		MinuteLimiter: rate.NewLimiter(rate.Every(time.Minute/30), 30),
		// does not cap burst since the burst pertains to the amount requested
		// within the interval of a day/14400 parts (~6sec)
		DayLimiter: rate.NewLimiter(rate.Every(time.Hour*24/14400), 14400),
	}
)

type tagContext struct {
	ctx    context.Context
	driver *sql.DB
	qry    *db.Queries
	client *genai.Client
	blobs  blob.Store
	model  Model
}

func (c tagContext) infer(content []*genai.Content) (res *genai.GenerateContentResponse, err error) {
	err = c.model.DayLimiter.Wait(c.ctx)
	if err != nil {
		return
	}
	err = c.model.MinuteLimiter.Wait(c.ctx)
	if err != nil {
		return
	}
	backoff := 4 * time.Second
	for {
		res, err = c.client.Models.GenerateContent(c.ctx, c.model.ID, content, nil)
		if err == nil {
			return
		}
		log.Println("gemini:", err)
		time.Sleep(backoff)
		backoff = min(2*backoff, 10*time.Minute)
	}
}

func (c tagContext) tag(r db.Resource) (err error) {
	if !r.Image.Valid {
		return
	}
	fmt.Println("tagging:", r.ID)

	f, err := c.blobs.Open(db.ToUint(r.Image.Int64))
	if err != nil {
		return
	}
	defer f.Close()

	buf := bytes.NewBuffer(nil)
	_, err = io.Copy(buf, f)
	if err != nil {
		return
	}

	parts := []*genai.Part{
		genai.NewPartFromText("Describe the subject of the image in as few words as possible. Format it as a plain-text title."),
		genai.NewPartFromBytes(buf.Bytes(), http.DetectContentType(buf.Bytes())),
	}
	res, err := c.infer([]*genai.Content{genai.NewContentFromParts(parts, "user")})
	if err != nil {
		return
	}
	name := res.Text()

	fmt.Println("tagged:", name, r.ID)

	err = c.qry.UpdateResource(c.ctx, db.UpdateResourceParams{
		ID:       r.ID,
		Name:     name,
		Type:     r.Type,
		Comments: r.Comments,
	})
	return
}

func (c tagContext) tagAll() (err error) {
	resources, err := c.qry.Search(c.ctx, "Untitled*")
	if err != nil {
		return
	}
	jobs := make(chan db.Resource)
	var errs []error
	var errMutex sync.Mutex
	wg := sync.WaitGroup{}
	wg.Add(runtime.NumCPU())
	for range runtime.NumCPU() {
		go func() {
			defer wg.Done()
			for {
				select {
				case <-c.ctx.Done():
					return
				case j, ok := <-jobs:
					if !ok { // if channel is closed
						return
					}
					err := c.tag(j)
					if err == nil {
						continue
					}
					errMutex.Lock()
					log.Println("error:", err)
					errs = append(errs, err)
					errMutex.Unlock()
				}
			}
		}()
	}
	for _, r := range resources {
		select {
		case <-c.ctx.Done():
			close(jobs)
			return errors.Join(errs...)
		case jobs <- r:
		}
	}
	close(jobs)
	wg.Wait()
	return errors.Join(errs...)
}

func main() {
	dataPath := flag.String("data", ".", "The directory in which to store item-archive data.")
	flag.Parse()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	driver, qry, err := db.Open(ctx, filepath.Join(*dataPath, "state.db"))
	if err != nil {
		log.Fatal(err)
	}
	defer driver.Close()
	client, err := genai.NewClient(ctx, nil)
	if err != nil {
		log.Fatal(err)
	}

	tagctx := tagContext{
		ctx:    ctx,
		driver: driver,
		qry:    qry,
		client: client,
		blobs:  blob.Store{Dir: filepath.Join(*dataPath, "blobs")},
		model:  gemma_3_27b_free,
	}
	err = tagctx.tagAll()
	if err != nil {
		log.Println(err)
		return
	}
}
