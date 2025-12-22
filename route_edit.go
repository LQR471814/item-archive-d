package main

import (
	"fmt"
	"html/template"
	"item-archive-d/internal/db"
	"net/http"
	"path"
)

type EditProps struct {
	Path        string
	Action      string
	Name        string
	Color       string
	Comments    string
	IsItem      bool
	IsContainer bool
}

const edit_template = `<!DOCTYPE html>
<html>
<head>
	<title>Item Archive: Editing {{.Path}}</title>
	<style>
	h1, h2, h3, h4, h5, h6 {
		margin: 0.75rem 0rem;
	}
	img {
		max-width: 80px;
		max-height: 150px;
	}
	</style>
</head>

<body>
	<a href="/">&lt;&lt; Cancel</a>

	<hr>

	<form action="{{.Action}}" method="post" enctype="multipart/form-data">
		<h4>Editing: {{.Path}}</h4>
		<div>
			<label for="name">Name:</label>
			<input type="text" name="name" id="name" placeholder="Resource name" value="{{.Name}}" required>
		</div>
		<div>
			<label for="color">Color:</label>
			<input type="text" name="color" id="color" placeholder="Physical color" value="{{.Color}}">
		</div>
		<div>
			<label for="comments">Comments:</label>
			<textarea name="comments" id="comments" placeholder="Comments">{{.Comments}}</textarea>
		</div>
		<div>
			<label for="image">Image:</label>
			<input type="file" name="image" id="image">
		</div>
		<div>
			<label for="type">Type:</label>
			<select name="type" id="type-select">
				<option value="item" {{if .IsItem}}selected{{end}}>Item</option>
				<option value="container" {{if .IsContainer}}selected{{end}}>Container</option>
			</select>
		</div>
		<input type="submit" value="Submit">
	</form>
</body>
</html>

`

func (c Context) Edit() (string, func(w http.ResponseWriter, r *http.Request)) {
	tmpl, err := template.New("edit").Parse(edit_template)
	if err != nil {
		panic(err)
	}
	return "/_edit/{path...}", c.withTx(func(txqry *db.Queries, w http.ResponseWriter, r *http.Request) (err error) {
		if r.Method != http.MethodGet {
			w.Write([]byte("unsupported method"))
			w.WriteHeader(400)
			return
		}
		ctx := r.Context()

		p := r.PathValue("path")
		id, err := txqry.Resolve(ctx, p)
		if err != nil {
			return
		}
		if !id.Valid {
			err = fmt.Errorf("unknown resource: %s", p)
			return
		}
		resource, err := txqry.GetResource(ctx, id.Int64)
		if err != nil {
			return
		}

		err = tmpl.Execute(w, EditProps{
			Path:        p,
			Action:      path.Join("/_update", p),
			Name:        resource.Name,
			Color:       resource.Color,
			Comments:    resource.Comments,
			IsItem:      resource.Type == "item",
			IsContainer: resource.Type == "container",
		})
		return
	})
}

func (c Context) Update() (string, func(w http.ResponseWriter, r *http.Request)) {
	return "/_update/{path...}", c.withTx(func(txqry *db.Queries, w http.ResponseWriter, r *http.Request) (err error) {
		if r.Method != http.MethodPost {
			w.Write([]byte("unsupported method"))
			w.WriteHeader(400)
			return
		}
		ctx := r.Context()

		p := r.PathValue("path")
		id, err := txqry.Resolve(ctx, p)
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

		err = txqry.UpdateResource(ctx, db.UpdateResourceParams{
			ID:       id.Int64,
			Name:     name,
			Type:     resourceType,
			Color:    color,
			Comments: comments,
		})
		if err != nil {
			return
		}

		image := first(r.MultipartForm.File, "image")
		imageID, err := handleImageUpload(c.blobs, image)
		if err != nil {
			return
		}
		if imageID.Valid {
			err = txqry.UpdateResourceImage(ctx, db.UpdateResourceImageParams{
				ID:    id.Int64,
				Image: imageID,
			})
			if err != nil {
				return
			}
		}

		w.Header().Set("Location", path.Join("/", path.Dir(p))+"/")
		w.WriteHeader(303)
		return
	})
}
