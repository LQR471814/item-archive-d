package main

import (
	"context"
	"database/sql"
	"flag"
	"item-archive-d/internal/blob"
	"item-archive-d/internal/db"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"time"
)

// All methods on Context will be registered as HTTP routes according to the
// pattern they specify in their doc comment
type Context struct {
	driver *sql.DB
	qry    *db.Queries
	blobs  blob.Store
}

func main() {
	addr := flag.String("addr", ":4502", "The address to listen on.")
	dataPath := flag.String("data", ".", "The directory in which to store item-archive data.")
	migrationPath := flag.String("migration", "", "Specify a file containing migration statements to run upon opening the database. (optional)")
	flag.Parse()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	migrations := ""
	if *migrationPath != "" {
		log.Println("using migrations:", *migrationPath)
		migrationContents, err := os.ReadFile(*migrationPath)
		if err != nil {
			log.Println(err)
			return
		}
		migrations = string(migrationContents)
	}

	driver, qry, err := db.Open(ctx, filepath.Join(*dataPath, "state.db"), migrations)
	if err != nil {
		log.Println(err)
		return
	}

	mux := http.NewServeMux()
	router := Context{
		driver: driver,
		qry:    qry,
		blobs:  blob.Store{Dir: filepath.Join("blobs")},
	}
	mux.HandleFunc(router.Search())
	mux.HandleFunc(router.Image())
	mux.HandleFunc(router.Edit())
	mux.HandleFunc(router.Update())
	mux.HandleFunc(router.MoveStart())
	mux.HandleFunc(router.MoveFinish())
	mux.HandleFunc(router.DeleteConfirm())
	mux.HandleFunc(router.DeleteShallow())
	mux.HandleFunc(router.DeleteDeep())
	mux.HandleFunc(router.List())

	srv := &http.Server{
		Addr:    *addr,
		Handler: mux,
	}
	go func() {
		err := srv.ListenAndServe()
		if err != nil {
			log.Println("listen: ", err)
		}
	}()

	log.Println("serving on...", *addr)
	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Println("server shutdown:", err)
	}
}
