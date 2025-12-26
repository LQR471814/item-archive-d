package main

import (
	"database/sql"
	"fmt"
	"html/template"
	"item-archive-d/internal/db"
	"net/http"
	"path"
	"strings"
)

type MoveConfirmProps struct {
	Path              string
	SubtreeEntries    []string
	SubtreeContainers []string
	Cancel            string
	FinishHref        string
}

const move_start_template = `<!DOCTYPE html>
<html>
<head>
	<meta name="viewport" content="width=device-width, initial-scale=1.0">
	<title>Item Archive: Moving items in {{.Path}}</title>
	<style>
	h1, h2, h3, h4, h5, h6 {
		margin: 0.75rem 0rem;
	}
	* {
		box-sizing: border-box;
	}
	body {
		margin: 0;
		padding: 0.5rem;
	}
	img {
		max-width: 80px;
		max-height: 150px;
	}
	</style>
</head>

<body style="position: relative">
	<div style="background-color: white; position: sticky; top: 0;">
		<a href="{{.Cancel}}">&lt;&lt; Cancel</a>
		<hr>
	</div>

	<form action="{{.FinishHref}}" method="post">
		<h4>Moving items</h4>
		{{range .SubtreeEntries}}
			<div>
				<input type="checkbox" name="{{.}}" value="">
				<span>{{.}}</span>
			</div>
		{{end}}
		<div>
			<label for="to">To:</label>
			<input style="width: 90%" list="targets" id="to" name="__to__" placeholder="Type to search...">
			<datalist id="targets">
				{{range .SubtreeContainers}}
					<option value="{{.}}">
				{{end}}
			</datalist>
		</div>
		<input type="submit" value="Submit">
	</form>
</body>
</html>`

func (c Context) MoveStart() (string, func(w http.ResponseWriter, r *http.Request)) {
	tmpl, err := template.New("move-start").Parse(move_start_template)
	if err != nil {
		panic(err)
	}
	return "/_move_start/{path...}", c.withTx(func(txqry *db.Queries, w http.ResponseWriter, r *http.Request) (err error) {
		ctx := r.Context()
		p := r.PathValue("path")

		id, err := txqry.Resolve(ctx, p)
		if err != nil {
			return
		}
		var subtree []string
		if !id.Valid {
			if p != "" && p != "/" {
				err = fmt.Errorf("unknown resource: %s", p)
				return
			}
			subtree, err = c.qry.GetFullTree(ctx)
		} else {
			subtree, err = c.qry.GetSubtree(ctx, id.Int64)
		}
		if err != nil {
			return
		}
		containers, err := c.qry.GetAllContainers(ctx)
		if err != nil {
			return
		}
		containers = append([]string{"/"}, containers...)

		err = tmpl.Execute(w, MoveConfirmProps{
			Path:              p,
			SubtreeEntries:    subtree,
			SubtreeContainers: containers,
			Cancel:            trailingPath(path.Join("/", p)),
			FinishHref:        trailingPath(path.Join("/_move_finish", p)),
		})
		return
	})
}

func hasAncestor(ancestor, test string) bool {
	ancestorSegments := strings.Split(ancestor, "/")
	testSegments := strings.Split(test, "/")
	i := 0
	j := 0
	for i < len(ancestorSegments) {
		if j >= len(testSegments) {
			return false
		}
		a := ancestorSegments[i]
		t := testSegments[j]
		if a == "" {
			i++
			continue
		}
		if t == "" {
			j++
			continue
		}
		if a == t {
			i++
			j++
			continue
		}
		return false
	}
	return true
}

func (c Context) MoveFinish() (string, func(w http.ResponseWriter, r *http.Request)) {
	return "/_move_finish/{path...}", c.withTx(func(txqry *db.Queries, w http.ResponseWriter, r *http.Request) (err error) {
		if r.Method != http.MethodPost {
			w.WriteHeader(400)
			w.Write([]byte("unsupported method"))
			return
		}

		ctx := r.Context()
		p := r.PathValue("path")
		id, err := txqry.Resolve(ctx, p)
		if err != nil {
			return
		}
		if !id.Valid && p != "" && p != "/" {
			w.WriteHeader(400)
			w.Write([]byte("Cannot delete non-existant resource."))
			return
		}

		err = r.ParseForm()
		if err != nil {
			return
		}
		to := r.Form.Get("__to__")
		toId, err := txqry.Resolve(ctx, to)
		if err != nil {
			return
		}

		ids := []int64{}
		for relative := range r.Form {
			if relative == "__to__" {
				continue
			}
			fullpath := path.Join(p, relative)

			// abort if: the destination (to) is a child of any of the
			// resources being moved
			if hasAncestor(fullpath, to) {
				w.WriteHeader(400)
				fmt.Fprintf(w, "cannot move resource '%s' into its own subtree '%s'", fullpath, to)
				return
			}

			var resolved sql.NullInt64
			resolved, err = txqry.Resolve(ctx, fullpath)
			if err != nil {
				return
			}
			if !resolved.Valid {
				err = fmt.Errorf("unknown resource: %s", fullpath)
				return
			}
			ids = append(ids, resolved.Int64)
		}

		err = txqry.MoveResources(ctx, db.MoveResourcesParams{
			Ids:       ids,
			NewParent: toId,
		})
		if err != nil {
			return
		}

		w.Header().Set("Location", trailingPath(path.Join("/", to)))
		w.WriteHeader(303)
		return
	})
}
