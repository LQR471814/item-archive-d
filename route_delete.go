package main

import (
	"database/sql"
	"html/template"
	"item-archive-d/internal/db"
	"net/http"
	"path"
)

type DeleteConfirmProps struct {
	Path          string
	Cancel        string
	ActionShallow string
	ActionDeep    string
}

const delete_confirm_template = `<!DOCTYPE html>
<html>
<head>
	<meta name="viewport" content="width=device-width, initial-scale=1.0">
	<title>Item Archive: Deleting {{.Path}}</title>
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

<body>
	<a href="{{.Cancel}}">&lt;&lt; Cancel</a>

	<hr>

	<form method="post">
		<h4>Deleting: {{.Path}}</h4>
		<input formaction="{{.ActionDeep}}" type="submit" value="Delete resource and children">
		<input formaction="{{.ActionShallow}}" type="submit" value="Delete resource, keep children">
	</form>
</body>
</html>`

func (c Context) DeleteConfirm() (string, func(w http.ResponseWriter, r *http.Request)) {
	tmpl, err := template.New("delete-confirm").Parse(delete_confirm_template)
	if err != nil {
		panic(err)
	}
	return "/_delete_confirm/{path...}", c.withError(func(w http.ResponseWriter, r *http.Request) (err error) {
		p := r.PathValue("path")
		err = tmpl.Execute(w, DeleteConfirmProps{
			Path:          p,
			ActionDeep:    path.Join("/_delete_deep", p),
			ActionShallow: path.Join("/_delete_shallow", p),
			Cancel:        trailingPath(path.Join("/", path.Dir(p))),
		})
		return
	})
}

func (c Context) DeleteShallow() (string, func(w http.ResponseWriter, r *http.Request)) {
	return "/_delete_shallow/{path...}", c.withTx(func(txqry *db.Queries, w http.ResponseWriter, r *http.Request) (err error) {
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
		if !id.Valid {
			w.WriteHeader(400)
			w.Write([]byte("Cannot delete non-existant resource."))
			return
		}
		resource, err := txqry.GetResource(ctx, id.Int64)
		if err != nil {
			return
		}
		err = txqry.ChangeParent(ctx, db.ChangeParentParams{
			NewParent: resource.ParentID,
			OldParent: sql.NullInt64{Int64: resource.ID, Valid: true},
		})
		if err != nil {
			return
		}
		err = txqry.DeleteResource(ctx, id.Int64)
		if err != nil {
			return
		}
		w.Header().Set("Location", trailingPath(path.Join("/", path.Dir(p))))
		w.WriteHeader(303)
		return
	})
}

func (c Context) DeleteDeep() (string, func(w http.ResponseWriter, r *http.Request)) {
	return "/_delete_deep/{path...}", c.withTx(func(txqry *db.Queries, w http.ResponseWriter, r *http.Request) (err error) {
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
		if !id.Valid {
			w.WriteHeader(400)
			w.Write([]byte("Cannot delete non-existant resource."))
			return
		}
		err = txqry.DeleteResource(ctx, id.Int64)
		if err != nil {
			return
		}
		w.Header().Set("Location", trailingPath(path.Join("/", path.Dir(p))))
		w.WriteHeader(303)
		return
	})
}
