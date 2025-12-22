package main

import (
	"item-archive-d/internal/db"
	"net/http"
	"path"
)

func (c Context) Delete() (string, func(w http.ResponseWriter, r *http.Request)) {
	return "/_delete/{path...}", c.withTx(func(txqry *db.Queries, w http.ResponseWriter, r *http.Request) (err error) {
		p := r.PathValue("path")
		id, err := txqry.Resolve(r.Context(), p)
		if err != nil {
			return
		}
		if !id.Valid {
			w.WriteHeader(400)
			w.Write([]byte("Cannot delete non-existant resource."))
			return
		}
		err = txqry.DeleteResource(r.Context(), id.Int64)
		if err != nil {
			return
		}
		w.Header().Set("Location", trailingPath(path.Dir(p)))
		w.WriteHeader(303)
		return
	})
}
