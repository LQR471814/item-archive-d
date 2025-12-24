package main

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"item-archive-d/internal/blob"
	"item-archive-d/internal/db"
	"log"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"sync"
	"time"

	"golang.org/x/time/rate"
	"google.golang.org/genai"
)

type tagContext struct {
	ctx     context.Context
	driver  *sql.DB
	qry     *db.Queries
	client  *genai.Client
	blobs   blob.Store
	limiter *rate.Limiter
}

func (c tagContext) infer(model string, content []*genai.Content) (res *genai.GenerateContentResponse, err error) {
	err = c.limiter.Wait(c.ctx)
	if err != nil {
		return
	}
	backoff := 4 * time.Second
	for {
		res, err = c.client.Models.GenerateContent(c.ctx, model, content, nil)
		if err == nil {
			return
		}
		log.Println("gemini:", err)
		time.Sleep(backoff)
		backoff = min(2*backoff, 10*time.Minute)
	}
}

func (c tagContext) tag(r db.Resource) (err error) {
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
	res, err := c.infer("gemini-2.5-flash", []*genai.Content{genai.NewContentFromParts(parts, "user")})
	if err != nil {
		return
	}
	name := res.Text()

	fmt.Println("tagged:", name)

	err = c.qry.UpdateResource(c.ctx, db.UpdateResourceParams{
		ID:       r.ID,
		Name:     name,
		Type:     r.Type,
		Color:    r.Color,
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
			for j := range jobs {
				err := c.tag(j)
				errMutex.Lock()
				if err != nil {
					log.Println("error:", err)
				}
				errs = append(errs, err)
				errMutex.Unlock()
			}
		}()
	}
	for _, r := range resources {
		jobs <- r
	}
	close(jobs)
	wg.Wait()
	return errors.Join(errs...)
}

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	driver, qry, err := db.Open(ctx)
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
		blobs:  blob.Store{Dir: "blobs"},
		// burst 1 with max 5 requests per minute
		limiter: rate.NewLimiter(rate.Every(time.Minute/5), 1),
	}
	err = tagctx.tagAll()
	if err != nil {
		log.Println(err)
		return
	}
}
