package main

import (
	"database/sql"
	"html/template"
	"item-archive-d/internal/db"
	"net/http"
	"path"
	"strconv"
	"strings"
)

type SearchProps_Row struct {
	IsItem     bool
	Name       string
	NameHref   string
	ParentHref string
	Color      string
	Comments   string
	ImageSrc   sql.NullString
}

type SearchProps struct {
	Query string
	Rows  []SearchProps_Row
}

const search_template = `<!DOCTYPE html>
<html>
<head>
	<meta name="viewport" content="width=device-width, initial-scale=1.0">
	<title>Item Archive: Search {{.Query}}</title>
	<style>
	* {
		box-sizing: border-box;
	}
	th, td {
		text-align: start;
		padding-right: 1rem;
	}
	h1, h2, h3, h4, h5, h6, hr, body {
		margin: 0;
	}
	img {
		max-width: 80px;
		max-height: 150px;
	}
	html, body {
		height: 100%;
	}
	body {
		padding: 0.5rem;
	}
	.flex-horizontal {
		display: flex;
		gap: 0.5rem;
	}
	.flex-vertical {
		display: flex;
		flex-direction: column;
		gap: 0.5rem;
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

<body class="flex-vertical">
	<a href="/">&lt;&lt; Home</a>

	<hr>

	<form class="flex-vertical" action="/_search" method="get">
		<h4>Search results: '{{.Query}}'</h4>
		<div class="flex-horizontal">
			<input type="text" name="q" id="q" value="{{.Query}}" placeholder="Search query..." required autofocus>
			<input type="submit" value="Submit">
		</div>
	</form>

	<hr>

	<div style="height: 100%; overflow-y: auto;">
		<table>
			<thead style="position: sticky; top: 0; background: white;">
				<th>Parent</th>
				<th>Name</th>
				<th>Color</th>
				<th>Comments</th>
				<th>Image</th>
			</thead>
			<tbody>
				{{range .Rows}}
				<tr>
					<td><a href="{{.ParentHref}}">{{.ParentHref}}</a></td>
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
				</tr>
				{{end}}
			</tbody>
		</table>
	</div>
</body>
</html>
`

func (c Context) Search() (string, func(w http.ResponseWriter, r *http.Request)) {
	tmpl, err := template.New("list").Parse(search_template)
	if err != nil {
		panic(err)
	}
	return "/_search", c.withTx(func(txqry *db.Queries, w http.ResponseWriter, r *http.Request) (err error) {
		ctx := r.Context()
		err = r.ParseForm()
		if err != nil {
			return
		}
		query := r.Form.Get("q")
		resources, err := txqry.Search(ctx, query)
		if err != nil {
			return
		}
		rows := make([]SearchProps_Row, len(resources))
		for i, r := range resources {
			var segments []string
			segments, err = txqry.GetPath(ctx, r.ID)
			if err != nil {
				return
			}
			fullPath := trailingPath(strings.Join(segments, "/"))
			parent := trailingPath(strings.Join(segments[:len(segments)-1], "/"))

			rows[i] = SearchProps_Row{
				IsItem:     r.Type == "item",
				Name:       r.Name,
				NameHref:   fullPath,
				ParentHref: parent,
				Color:      r.Color,
				Comments:   r.Comments,
			}
			if r.Image.Valid {
				id := strconv.FormatUint(toUint(r.Image.Int64), 10)
				rows[i].ImageSrc = sql.NullString{
					String: path.Join("/_image", id),
					Valid:  true,
				}
			}
		}
		err = tmpl.Execute(w, SearchProps{
			Query: query,
			Rows:  rows,
		})
		return
	})
}
