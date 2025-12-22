package main

import (
	"database/sql"
	"html/template"
	"net/http"
	"path"
	"strconv"
)

type SearchProps_Row struct {
	Name     string
	NameHref string
	Color    string
	Comments string
	ImageSrc sql.NullString
}

type SearchProps struct {
	Query string
	Rows  []SearchProps_Row
}

const search_template = `<!DOCTYPE html>
<html>
<head>
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
				<th>Name</th>
				<th>Color</th>
				<th>Comments</th>
				<th>Image</th>
			</thead>
			<tbody>
				{{range .Rows}}
				<tr>
					<td><a href="{{.NameHref}}">{{.Name}}/</a></td>
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
	return "/_search", withError(func(w http.ResponseWriter, r *http.Request) (err error) {
		err = r.ParseForm()
		if err != nil {
			return
		}
		query := r.Form.Get("q")
		resources, err := c.qry.Search(r.Context(), query)
		if err != nil {
			return
		}
		rows := make([]SearchProps_Row, len(resources))
		for i, r := range resources {
			rows[i] = SearchProps_Row{
				Name:     r.Name,
				NameHref: path.Join("/_resource", strconv.FormatUint(toUint(r.ID), 10)),
				Color:    r.Color.String,
				Comments: r.Comments.String,
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
