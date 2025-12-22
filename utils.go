package main

import (
	"database/sql"
	"item-archive-d/internal/blob"
	"item-archive-d/internal/db"
	"log"
	"mime/multipart"
	"net/http"
	"unsafe"
)

func (c Context) withTx(fn func(txqry *db.Queries, w http.ResponseWriter, r *http.Request) error) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		tx, err := c.driver.BeginTx(ctx, nil)
		if err != nil {
			w.WriteHeader(500)
			w.Write([]byte(err.Error()))
			log.Println(err)
			return
		}
		defer tx.Rollback()
		txqry := c.qry.WithTx(tx)
		err = fn(txqry, w, r)
		if err != nil {
			w.WriteHeader(500)
			w.Write([]byte(err.Error()))
			log.Println(err)
			return
		}
		err = tx.Commit()
		if err != nil {
			w.WriteHeader(500)
			w.Write([]byte(err.Error()))
			log.Println(err)
			return
		}
	}
}

func (c Context) withError(fn func(w http.ResponseWriter, r *http.Request) error) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		err := fn(w, r)
		if err != nil {
			w.Write([]byte(err.Error()))
			log.Println(err)
			return
		}
	}
}

// trailingPath adds a trailing '/' to the given path if it does not already
// exist
func trailingPath(p string) string {
	if p == "" {
		return "/"
	}
	if p[len(p)-1] == '/' {
		return p
	}
	return p + "/"
}

// toUint converts an integer to a uint without changing the bytes
func toUint(i int64) uint64 {
	return *(*uint64)(unsafe.Pointer(&i))
}

// toInt converts a uint to an int without changing the bytes
func toInt(i uint64) int64 {
	return *(*int64)(unsafe.Pointer(&i))
}

func handleImageUpload(blobs blob.Store, img *multipart.FileHeader) (id sql.NullInt64, err error) {
	if img == nil {
		return
	}
	var file multipart.File
	file, err = img.Open()
	if err != nil {
		return
	}
	var imageId uint64
	imageId, err = blobs.Store(file)
	if err != nil {
		return
	}
	id = sql.NullInt64{Int64: toInt(imageId), Valid: true}
	return
}

func first[T any](m map[string][]T, key string) T {
	list, ok := m[key]
	if !ok {
		var zero T
		return zero
	}
	if len(list) == 0 {
		var zero T
		return zero
	}
	return list[0]
}
