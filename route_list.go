package main

import (
	"database/sql"
	"errors"
	"fmt"
	"html/template"
	"item-archive-d/internal/db"
	"net/http"
	"path"
	"strconv"
	"strings"
)

type ListProps_PathSegment struct {
	Location string
	Name     string
}

type ListProps_Row struct {
	IsItem     bool
	Name       string
	NameHref   string
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
	<meta name="viewport" content="width=device-width, initial-scale=1.0">
	<title>Item Archive: {{.Path}}</title>
	<style>
	th, td {
		text-align: start;
		padding-right: 0.5rem;
	}
	h1, h2, h3, h4, h5, h6 {
		margin: 0.75rem 0rem;
	}
	img {
		max-width: 80px;
		max-height: 150px;
	}
	@media (max-width: 768px) { /* prevent zoom on text-input for mobile */
		input,
		textarea,
		select {
			font-size: 16px;
		}
	}
	</style>
</head>

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
		</thead>
		<tbody>
			{{if $.IsNotRoot}}
				<tr>
					<td><a href="..">../</a></td>
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
				<td><a href={{.EditHref}}>Edit</a> / <a href="{{.DeleteHref}}">Delete</a></td>
			</tr>
			{{end}}
		</tbody>
	</table>

	<hr>

	<form action="" method="post" enctype="multipart/form-data">
		<h4>New Item</h4>
		<div>
			<label for="name">Name:</label>
			<input type="text" name="name" id="name" placeholder="Resource name">
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
			Location: trailingPath(accumulated),
			Name:     s,
		})
	}
	return
}

func (c Context) List() (string, func(w http.ResponseWriter, r *http.Request)) {
	tmpl, err := template.New("list").Parse(list_template)
	if err != nil {
		panic(err)
	}
	return "/", c.withTx(func(txqry *db.Queries, w http.ResponseWriter, r *http.Request) (err error) {
		ctx := r.Context()
		p := path.Join("/", r.URL.Path)

		parentID, err := txqry.Resolve(ctx, p)
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

			var imageID sql.NullInt64
			imageID, err = handleImageUpload(c.blobs, image)
			if err != nil {
				return
			}

			if name == "" {
				var resources []db.Resource
				resources, err = txqry.ListResources(ctx, parentID)
				if err != nil {
					return
				}
				maxIdx := uint64(0)
				for _, r := range resources {
					if !strings.HasPrefix(r.Name, "Untitled") {
						continue
					}
					segments := strings.Split(r.Name, " ")
					if len(segments) != 2 {
						continue
					}
					idx, err := strconv.ParseUint(segments[1], 10, 64)
					if err != nil {
						continue
					}
					if idx > uint64(maxIdx) {
						maxIdx = idx
					}
				}
				name = fmt.Sprintf("Untitled %d", maxIdx+1)
			}

			_, err = txqry.CreateResource(ctx, db.CreateResourceParams{
				ParentID: parentID,
				Name:     name,
				Color:    color,
				Type:     resourceType,
				Comments: comments,
				Image:    imageID,
			})
			if err != nil {
				return
			}
			w.Header().Set("Location", r.URL.Path)
			w.WriteHeader(303)
			return
		}

		rows, err := txqry.ListResources(ctx, parentID)
		if err != nil {
			return
		}

		listRows := make([]ListProps_Row, len(rows))
		for i, r := range rows {
			listRows[i] = ListProps_Row{
				Name: r.Name,
				// trailing slash must be included, otherwise ".." href breaks
				NameHref:   trailingPath(path.Join(p, r.Name)),
				IsItem:     r.Type == "item",
				Color:      r.Color,
				Comments:   r.Comments,
				EditHref:   path.Join("/_edit", p, r.Name),
				DeleteHref: path.Join("/_delete_confirm", p, r.Name),
			}
			if r.Image.Valid {
				id := strconv.FormatUint(toUint(r.Image.Int64), 10)
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
