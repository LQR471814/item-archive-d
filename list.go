package main

import (
	"database/sql"
	"errors"
	"fmt"
	"html/template"
	"item-archive-d/internal/db"
	"mime/multipart"
	"net/http"
	"path"
	"strconv"
	"strings"
	"unsafe"
)

type ListProps_PathSegment struct {
	Location string
	Name     string
}

type ListProps_Row struct {
	IsItem     bool
	Name       string
	NameHref   string
	Type       string
	Color      string
	Comments   string
	ImageSrc   sql.NullString
	EditHref   string
	DeleteHref string
}

type ListProps struct {
	IsNotRoot    bool
	Path         string
	PathSegments []ListProps_PathSegment
	Rows         []ListProps_Row
}

const list_template = `<!DOCTYPE html>
<html>
<head>
	<title>Item Archive: {{.Path}}</title>
	<style>
	th, td {
		text-align: start;
		padding-right: 1rem;
	}
	h1, h2, h3, h4, h5, h6 {
		margin: 0.75rem 0rem;
	}
	img {
		max-width: 80px;
		max-height: 150px;
	}
	</style>
</head>

{{define "form"}}
	<form action="" method="post" enctype="multipart/form-data">
		<h4>{{.}}</h4>
		<div>
			<label for="name">Name:</label>
			<input type="text" name="name" id="name" placeholder="Resource name" required>
		</div>
		<div>
			<label for="color">Color:</label>
			<input type="text" name="color" id="color" placeholder="Physical color">
		</div>
		<div>
			<label for="comments">Comments:</label>
			<textarea name="comments" id="comments" placeholder="Comments"></textarea>
		</div>
		<div>
			<label for="image">Image:</label>
			<input type="file" name="image" id="image">
		</div>
		<div>
			<label for="type">Type:</label>
			<select name="type" id="type-select">
				<option value="item" selected>Item</option>
				<option value="container">Container</option>
			</select>
		</div>
		<input type="submit" value="Submit">
	</form>
{{end}}

<body>
	<div style="display: flex; gap: 0.1rem; flex-wrap: wrap;">
		<a href="/">&lt;home&gt;</a>
		<span>/</span>
		{{range .PathSegments}}
		<a href="{{.Location}}">{{.Name}}</a>
		<span>/</span>
		{{end}}
	</div>

	<hr>

	<div>
		<form action="/_search" method="get">
			<h4><label for="q">Search</label></h4>
			<input type="text" name="q" id="q" placeholder="Search query..." required>
			<input type="submit" value="Submit">
		</form>
	</div>

	<hr>

	<table>
		<thead>
			<th>Name</th>
			<th>Color</th>
			<th>Comments</th>
			<th>Image</th>
			<th></th>
			<th></th>
		</thead>
		<tbody>
			{{if $.IsNotRoot}}
				<tr>
					<td><a href="..">../</a></td>
					<td></td>
					<td></td>
					<td></td>
					<td></td>
					<td></td>
				</tr>
			{{end}}
			{{range .Rows}}
			<tr>
				{{if .IsItem}}
					<td>{{.Name}}</td>
				{{else}}
					<td><a href="{{.NameHref}}">{{.Name}}/</a></td>
				{{end}}
				<td>{{.Color}}</td>
				<td>{{.Comments}}</td>
				<td>
					{{if .ImageSrc.Valid}}
						<img src="{{.ImageSrc.String}}" alt="Image of {{.Name}}">
					{{end}}
				</td>
				<td><a href={{.EditHref}}>Edit</a></td>
				<td><a href="{{.DeleteHref}}">Delete</a></td>
			</tr>
			{{end}}
		</tbody>
	</table>

	<hr>

	{{template "form" "New Item"}}
</body>
</html>`

func makePathSegments(p string) (out []ListProps_PathSegment) {
	segments := strings.Split(p, "/")
	accumulated := "/"
	for _, s := range segments {
		if s == "" {
			continue
		}
		accumulated = path.Join(accumulated, s)
		out = append(out, ListProps_PathSegment{
			// trailing slash must be included, otherwise ".." href breaks
			Location: accumulated + "/",
			Name:     s,
		})
	}
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

func (c Context) List() (string, func(w http.ResponseWriter, r *http.Request)) {
	tmpl, err := template.New("list").Parse(list_template)
	if err != nil {
		panic(err)
	}
	return "/", withError(func(w http.ResponseWriter, r *http.Request) (err error) {
		p := path.Join("/", r.URL.Path)

		parentID, err := c.qry.Resolve(r.Context(), p)
		if errors.Is(err, sql.ErrNoRows) {
			err = fmt.Errorf("unknown resource: %s", r.URL.Path)
			return
		}
		if err != nil {
			return
		}

		if r.Method == http.MethodPost {
			err = r.ParseMultipartForm(10 * 1000 * 1000 * 1000)
			if err != nil {
				return
			}

			name := first(r.MultipartForm.Value, "name")
			color := first(r.MultipartForm.Value, "color")
			resourceType := first(r.MultipartForm.Value, "type")
			comments := first(r.MultipartForm.Value, "comments")
			image := first(r.MultipartForm.File, "image")

			var imageField sql.NullInt64
			if image != nil {
				var file multipart.File
				file, err = image.Open()
				if err != nil {
					return
				}
				var imageId uint64
				imageId, err = c.blobs.Store(file)
				if err != nil {
					return
				}
				// cast directly into int64 without changing the underlying bytes
				imageField = sql.NullInt64{Int64: *(*int64)(unsafe.Pointer(&imageId)), Valid: true}
			}

			_, err = c.qry.CreateResource(r.Context(), db.CreateResourceParams{
				ParentID: parentID,
				Name:     name,
				Color:    sql.NullString{String: color, Valid: true},
				Type:     sql.NullString{String: resourceType, Valid: true},
				Comments: sql.NullString{String: comments, Valid: true},
				Image:    imageField,
			})
			if err != nil {
				return
			}
			w.Header().Set("Location", r.URL.Path)
			w.WriteHeader(303)
			return
		}

		rows, err := c.qry.ListResources(r.Context(), parentID)
		if err != nil {
			return
		}

		listRows := make([]ListProps_Row, len(rows))
		for i, r := range rows {
			listRows[i] = ListProps_Row{
				Name: r.Name,
				// trailing slash must be included, otherwise ".." href breaks
				NameHref:   path.Join(p, r.Name) + "/",
				IsItem:     r.Type.String == "item",
				Type:       r.Type.String,
				Color:      r.Color.String,
				Comments:   r.Comments.String,
				EditHref:   path.Join("/_edit", p, r.Name),
				DeleteHref: path.Join("/_delete", p, r.Name),
			}
			if r.Image.Valid {
				id := strconv.FormatUint(*(*uint64)(unsafe.Pointer(&r.Image.Int64)), 10)
				listRows[i].ImageSrc = sql.NullString{
					String: path.Join("/_image", id),
					Valid:  true,
				}
			}
		}

		err = tmpl.Execute(w, ListProps{
			IsNotRoot:    p != "/",
			Path:         p,
			PathSegments: makePathSegments(p),
			Rows:         listRows,
		})
		return
	})
}
