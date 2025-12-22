package main

import (
	"net/http"
	"strconv"
	"strings"
)

func (c Context) ResourceResolve() (string, func(w http.ResponseWriter, r *http.Request)) {
	return "/_resource/{id}", withError(func(w http.ResponseWriter, r *http.Request) (err error) {
		id, err := strconv.ParseUint(r.PathValue("id"), 10, 64)
		if err != nil {
			return
		}
		path, err := c.qry.GetPath(r.Context(), id)
		if err != nil {
			return
		}
		w.Header().Set("Location", "/"+strings.Join(path, "/"))
		w.WriteHeader(303)
		return
	})
}
