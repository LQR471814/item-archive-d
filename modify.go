package main

import (
	"database/sql"
	"fmt"
	"item-archive-d/internal/db"
	"net/http"
	"path"
)

func (c Context) Update() (string, func(w http.ResponseWriter, r *http.Request)) {
	return "/{path...}/_update", withError(func(w http.ResponseWriter, r *http.Request) (err error) {
		if r.Method != "POST" {
			w.Write([]byte("unsupported method"))
			w.WriteHeader(400)
			return
		}

		p := r.PathValue("path")
		id, err := c.qry.Resolve(r.Context(), p)
		if err != nil {
			return
		}
		if !id.Valid {
			err = fmt.Errorf("unknown resource")
			return
		}

		err = r.ParseMultipartForm(10 * 1000 * 1000 * 1000)
		if err != nil {
			return
		}

		name := first(r.MultipartForm.Value, "name")
		resourceType := first(r.MultipartForm.Value, "type")
		color := first(r.MultipartForm.Value, "color")
		comments := first(r.MultipartForm.Value, "comments")
		image := first(r.MultipartForm.File, "image")

		var imageID sql.NullInt64
		imageID, err = handleImageUpload(c.blobs, image)
		if err != nil {
			return
		}

		err = c.qry.UpdateResource(r.Context(), db.UpdateResourceParams{
			ID:       id.Int64,
			Name:     name,
			Type:     resourceType,
			Color:    color,
			Comments: comments,
			Image:    imageID,
		})
		if err != nil {
			return
		}
		w.Header().Set("Location", path.Join("/", path.Dir(p))+"/")
		w.WriteHeader(303)
		return
	})
}

func (c Context) Delete() (string, func(w http.ResponseWriter, r *http.Request)) {
	return "/_delete/{path...}", withError(func(w http.ResponseWriter, r *http.Request) (err error) {
		p := r.PathValue("path")
		id, err := c.qry.Resolve(r.Context(), p)
		if err != nil {
			return
		}
		if !id.Valid {
			w.Write([]byte("Cannot delete non-existant resource."))
			w.WriteHeader(400)
			return
		}
		err = c.qry.DeleteResource(r.Context(), id.Int64)
		if err != nil {
			return
		}
		w.Header().Set("Location", path.Join("/", path.Dir(p))+"/")
		w.WriteHeader(303)
		return
	})
}
