package main

import (
	"log"
	"net/http"
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
