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
	flag.Parse()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	driver, qry, err := db.Open(ctx)
	if err != nil {
		return
	}

	mux := http.NewServeMux()
	router := Context{
		driver: driver,
		qry:    qry,
		blobs:  blob.Store{Dir: "blobs"},
	}
	mux.HandleFunc(router.Search())
	mux.HandleFunc(router.Image())
	mux.HandleFunc(router.Delete())
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
