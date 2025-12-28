package db

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	_ "embed"

	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schema string

func Open(ctx context.Context, path, migrations string) (driver *sql.DB, qry *Queries, err error) {
	driver, err = sql.Open("sqlite", fmt.Sprintf(
		"file:%s?"+
			"_pragma=foreign_keys(1)&"+
			"_journal_mode=WAL&"+
			"_busy_timeout=5000",
		path,
	))
	if err != nil {
		return
	}
	err = driver.PingContext(ctx)
	if err != nil {
		return
	}
	qry = New(driver)

	if migrations != "" {
		_, err = driver.ExecContext(ctx, migrations)
		return
	}

	tx, err := driver.BeginTx(ctx, &sql.TxOptions{
		// no reads occur in the only statement executed
		Isolation: sql.LevelReadUncommitted,
	})
	if err != nil {
		return
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx, schema)
	if err == nil {
		err = tx.Commit()
		return
	}

	// if already setup
	if strings.Contains(err.Error(), "already exists") {
		err = nil
		return
	}

	// if some other error
	return
}
