package main

import (
	"io"
	"net/http"
	"strconv"
)

func (c Context) Image() (string, func(w http.ResponseWriter, r *http.Request)) {
	return "/_image/{id}", withError(func(w http.ResponseWriter, r *http.Request) (err error) {
		id, err := strconv.ParseUint(r.PathValue("id"), 10, 64)
		if err != nil {
			return
		}

		// specify that:
		// 1. public: can be cached by any cache (browser, cdn, proxy, etc...)
		// 2. max-age=1 year: cache for 1 year
		// 3. immutable: will not change during cache duration, specifies
		// additional requests to validate freshness are not necessary
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")

		f, err := c.blobs.Open(id)
		if err != nil {
			return
		}
		defer f.Close()
		_, err = io.Copy(w, f)
		if err != nil {
			return
		}
		return
	})
}
