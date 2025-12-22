package main

import (
	"log"
	"net/http"
	"path"
)

// func (c Context) EditCommit() (string, func(w http.ResponseWriter, r *http.Request)) {
// 	return "/{path...}/_edit/commit", func(w http.ResponseWriter, r *http.Request) {
// 		if r.Method != "POST" {
// 			w.Write([]byte("unsupported method"))
// 			w.WriteHeader(400)
// 			return
// 		}
//
// 		p := r.PathValue("path")
// 		id, err := c.qry.Resolve(r.Context(), p)
// 		if err != nil {
// 			w.Write([]byte(err.Error()))
// 			w.WriteHeader(500)
// 			log.Println(err)
// 			return
// 		}
//
// 		c.qry.UpdateResource(r.Context(), db.UpdateResourceParams{
// 			ID: id,
// 		})
// 		err := c.qry.PutResource(r.Context(), db.PutResourceParams{
// 			Name:   path.Base(p),
// 			Parent: path.Dir(p),
// 		})
// 		if err != nil {
// 			w.Write([]byte(err.Error()))
// 			w.WriteHeader(500)
// 			log.Println(err)
// 			return
// 		}
// 		r.URL.Path = path.Join("/", path.Dir(p))
// 		w.Header().Set("Location", r.URL.String())
// 		w.WriteHeader(303)
// 	}
// }

func (c Context) Delete() (string, func(w http.ResponseWriter, r *http.Request)) {
	return "/_delete/{path...}", func(w http.ResponseWriter, r *http.Request) {
		p := r.PathValue("path")
		id, err := c.qry.Resolve(r.Context(), p)
		if err != nil {
			w.Write([]byte(err.Error()))
			w.WriteHeader(500)
			log.Println(err)
			return
		}
		if !id.Valid {
			w.Write([]byte("Cannot delete non-existant resource."))
			w.WriteHeader(400)
			return
		}
		err = c.qry.DeleteResource(r.Context(), id.Int64)
		if err != nil {
			w.Write([]byte(err.Error()))
			w.WriteHeader(500)
			log.Println(err)
			return
		}
		r.URL.Path = path.Join("/", path.Dir(p))
		w.Header().Set("Location", r.URL.String())
		w.WriteHeader(303)
	}
}
