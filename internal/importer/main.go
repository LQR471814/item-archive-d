package main

import (
	"context"
	"database/sql"
	"item-archive-d/internal/db"
	"log"
	"os"
	"path/filepath"
	"strings"
)

func normalizeName(name string) string {
	segments := strings.Split(name, "_")
	for i, s := range segments {
		segments[i] = strings.ToUpper(s[0:1]) + s[1:]
	}
	return strings.Join(segments, " ")
}

func loadDir(ctx context.Context, txqry *db.Queries, cwd string, parentId int64) {
	entries, err := os.ReadDir(cwd)
	if err != nil {
		panic(err)
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		segments := strings.Split(e.Name(), ".")
		name := normalizeName(segments[0])
		macrotype := segments[len(segments)-1]
		tags := strings.Join(segments[1:len(segments)-1], ",")

		parent := sql.NullInt64{
			Int64: parentId,
			Valid: true,
		}
		if parentId < 0 {
			parent.Int64 = 0
			parent.Valid = false
		}
		id, err := txqry.CreateResource(ctx, db.CreateResourceParams{
			Name:     name,
			ParentID: parent,
			Type:     macrotype,
			Comments: tags,
		})
		if err != nil {
			panic(err)
		}
		loadDir(ctx, txqry, filepath.Join(cwd, e.Name()), id)
	}
}

func main() {
	ctx := context.Background()

	db, qry, err := db.Open(ctx, "state.db", "")
	if err != nil {
		log.Fatal(err)
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer tx.Rollback()
	txqry := qry.WithTx(tx)

	loadDir(ctx, txqry, ".", -1)

	err = tx.Commit()
	if err != nil {
		log.Fatal(err)
	}
}
