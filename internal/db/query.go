package db

import (
	"context"
	"database/sql"
	"strconv"
	"strings"
	"unsafe"
)

// ToUint converts an integer to a uint without changing the bytes
func ToUint(i int64) uint64 {
	return *(*uint64)(unsafe.Pointer(&i))
}

// ToInt converts a uint to an int without changing the bytes
func ToInt(i uint64) int64 {
	return *(*int64)(unsafe.Pointer(&i))
}

/*
This query effectively works like so:

1. You first construct a table "paths" that contains rows numbered
with an index and the path segment for the part of the path at
that index
2. You construct a table "found" that is defined as:

A base case (select statement):
- Look for the first row in the "paths" array.
- Set the value of the current_step column in all the rows found
to 1

The union statement:
- Essentially saying: "combine the rows in the base case above
with the rows in the recursive step below"

A recursive case (select statement):
- Join the rows whose parent_ids = the current found rows' ids
(essentially, join the child rows of the current found rows)
- Join the rows in "paths" whose name matches with the child
resources we just joined and whose current_step + 1 is exactly
equal to the step of the path. (essentially, join the paths with
the possible children candidates)
- Select just the resource id and current_step rows from the found
rows, increment the found child's current_step by 1.
*/

const resolve = `with recursive
	paths(step, name) as (
		values /*values*/
	),
	found as (
		select
			resource.id,
			1 as current_step
		from resource
		join paths on
			paths.name = resource.name
		where
			resource.parent_id is null and
			paths.step = 1

		union all

		select
			resource.id,
			found.current_step + 1
		from resource
		join found on
			resource.parent_id = found.id
		join paths on
			resource.name = paths.name and
			found.current_step + 1 = paths.step
	)

select id from found
where current_step = /*last_step*/
limit 1`

func (q *Queries) Resolve(ctx context.Context, path string) (out sql.NullInt64, err error) {
	var args []any
	var pathArgs strings.Builder
	i := 1
	for s := range strings.SplitSeq(path, "/") {
		if s == "" {
			continue
		}
		args = append(args, s)
		if i != 1 {
			pathArgs.WriteString(",")
		}
		pathArgs.WriteString("(")
		pathArgs.WriteString(strconv.Itoa(i))
		pathArgs.WriteString(",?)")
		i++
	}
	if pathArgs.String() == "" {
		return
	}
	// we only dynamically generate the part of the query with the argument
	// placeholders and then use the generated placeholders to call the query
	// to prevent sql injection from happening
	query := strings.Replace(resolve, "/*values*/", pathArgs.String(), 1)
	query = strings.Replace(query, "/*last_step*/", strconv.Itoa(len(args)), 1)

	row := q.db.QueryRowContext(ctx, query, args...)
	var id int64
	err = row.Scan(&id)
	if err != nil {
		return
	}
	out = sql.NullInt64{Int64: id, Valid: true}
	return
}

const getPath = `with recursive
	found as (
		select
			parent_id,
			name,
			1 as step
		from resource
		where resource.id = ?

		union all

		select
			resource.parent_id,
			resource.name,
			1 + found.step
		from resource
		join found
		where
			resource.id = found.parent_id
	)

select name from found
order by step desc`

func (q *Queries) GetPath(ctx context.Context, id int64) (path []string, err error) {
	rows, err := q.db.QueryContext(ctx, getPath, id)
	if err != nil {
		return
	}
	for rows.Next() {
		var name string
		err = rows.Scan(&name)
		if err != nil {
			return
		}
		path = append(path, name)
	}
	return
}

const search = `select resource.* from resource
join resource_fts on resource.id = resource_fts.rowid
where resource_fts match ?
order by rank`

func (q *Queries) Search(ctx context.Context, query string) ([]Resource, error) {
	rows, err := q.db.QueryContext(ctx, search, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []Resource
	for rows.Next() {
		var i Resource
		if err := rows.Scan(
			&i.ID,
			&i.ParentID,
			&i.Name,
			&i.Type,
			&i.Comments,
			&i.Image,
		); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

/*
we want to get a table of rows where each has one column: path of resource.
this table of rows should be the children (infinitely deep) from a given anchor
path

we select the anchor row, set path to /
we select the children rows, set path to anchor.path + children.name + "/"
*/
const getSubtree = `with recursive
	found as (
		select
			resource.id,
			'/' as path
		from resource
		where id = ?

		union all

		select
			resource.id,
			found.path || resource.name || '/'
		from resource
		join found on
			resource.parent_id = found.id
	)
select path from found`

func (q *Queries) GetSubtree(ctx context.Context, id int64) (out []string, err error) {
	rows, err := q.db.QueryContext(ctx, getSubtree, id)
	if err != nil {
		return
	}
	defer rows.Close()
	for rows.Next() {
		var p string
		err = rows.Scan(&p)
		if err != nil {
			return
		}
		out = append(out, p)
	}
	return
}

const getFullTree = `with recursive
	found as (
		select
			resource.id,
			'/' || resource.name || '/' as path
		from resource
		where parent_id is null

		union all

		select
			resource.id,
			found.path || resource.name || '/'
		from resource
		join found on
			resource.parent_id = found.id
	)
select path from found`

func (q *Queries) GetFullTree(ctx context.Context) (out []string, err error) {
	rows, err := q.db.QueryContext(ctx, getFullTree)
	if err != nil {
		return
	}
	defer rows.Close()
	for rows.Next() {
		var p string
		err = rows.Scan(&p)
		if err != nil {
			return
		}
		out = append(out, p)
	}
	return
}

const getAllContainers = `with recursive
	found as (
		select
			resource.id,
			resource.type,
			'/' || resource.name || '/' as path
		from resource
		where parent_id is null

		union all

		select
			resource.id,
			resource.type,
			found.path || resource.name || '/'
		from resource
		join found on
			resource.parent_id = found.id
	)
select path from found
where type = 'container'`

func (q *Queries) GetAllContainers(ctx context.Context) (out []string, err error) {
	rows, err := q.db.QueryContext(ctx, getAllContainers)
	if err != nil {
		return
	}
	defer rows.Close()
	for rows.Next() {
		var p string
		err = rows.Scan(&p)
		if err != nil {
			return
		}
		out = append(out, p)
	}
	return
}
