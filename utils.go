package main

import (
	"log"
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
