package main

import (
	"database/sql"
	"item-archive-d/internal/blob"
	"log"
	"mime/multipart"
	"net/http"
	"unsafe"
)

func withError(fn func(w http.ResponseWriter, r *http.Request) error) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		err := fn(w, r)
		if err != nil {
			w.Write([]byte(err.Error()))
			log.Println(err)
			return
		}
	}
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
