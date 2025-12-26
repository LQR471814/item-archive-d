package db

import (
	"database/sql"
	"strings"
)

/*
this details a model-based property testing program for the DB, ensuring that
all queries on the DB layer do not violate invariants of the DB state.

an property is simply a logical predicate which must be true for all possible
DB states.

ex. if exec command resource delete, preserve children -> said resource should
no longer exist but children should exist

or: if exec any command on resources other than delete commands -> all resources
should exist at the end of command

we will specify properties to check on each state of the DB we explore, then
explore the possible DB states by making queries stochastically

here we will simply use an in-memory map as an oracle for the DB
*/

type alreadyExistsErr struct{}

func (alreadyExistsErr) Error() string {
	return "row already exists"
}

type doesntExistErr struct{}

func (doesntExistErr) Error() string {
	return "row doesn't exist"
}

type resource struct {
	id       int64
	parent   sql.NullInt64
	name     string
	typeStr  string
	comments string
	image    int64
}

type oracle struct {
	resources map[int64]resource
	count     *int64
}

func newOracle() oracle {
	count := int64(1)
	return oracle{
		resources: make(map[int64]resource),
		count:     &count,
	}
}

func (o oracle) createResource(r resource) error {
	if r.id == 0 {
		r.id = *o.count
		*o.count++
	}
	_, ok := o.resources[r.id]
	if ok {
		return alreadyExistsErr{}
	}
	o.resources[r.id] = r
	return nil
}

type resourceUpdate struct {
	id       int64
	parent   *sql.NullInt64
	name     *string
	typeStr  *string
	comments *string
	image    *int64
}

func (o oracle) updateResource(r resourceUpdate) error {
	existing, ok := o.resources[r.id]
	if !ok {
		return doesntExistErr{}
	}
	if r.parent != nil {
		existing.parent = *r.parent
	}
	if r.name != nil {
		existing.name = *r.name
	}
	if r.typeStr != nil {
		existing.typeStr = *r.typeStr
	}
	if r.comments != nil {
		existing.comments = *r.comments
	}
	if r.image != nil {
		existing.image = *r.image
	}
	o.resources[r.id] = existing
	return nil
}

func (o oracle) moveResources(parentId sql.NullInt64, resources []int64) error {
	for _, id := range resources {
		existing, ok := o.resources[id]
		if !ok {
			return doesntExistErr{}
		}
		existing.parent = parentId
		o.resources[id] = existing
	}
	return nil
}

func (o oracle) changeParent(parentId, newParentId sql.NullInt64) {
	for _, r := range o.resources {
		if r.parent == parentId {
			r.parent = newParentId
			o.resources[r.id] = r
		}
	}
}

func (o oracle) listResources(parentId sql.NullInt64) (out []resource) {
	for _, r := range o.resources {
		if r.parent == parentId {
			out = append(out, r)
		}
	}
	return
}

func (o oracle) getResource(id int64) (resource, error) {
	existing, ok := o.resources[id]
	if !ok {
		return resource{}, doesntExistErr{}
	}
	return existing, nil
}

func (o oracle) deleteResource(id int64) {
	delete(o.resources, id)
}

func (o oracle) resolve(path string) (found sql.NullInt64, err error) {
	var current resource
	for name := range strings.SplitSeq(path, "/") {
		if name == "" {
			continue
		}
		for _, r := range o.resources {
			if r.parent == current.parent && r.name == name {
				current = r
				break
			}
		}
		err = doesntExistErr{}
		return
	}
	found = sql.NullInt64{Valid: true, Int64: current.id}
	return
}

func (o oracle) getPath(id int64) (p []string, err error) {
	existing, ok := o.resources[id]
	if !ok {
		err = doesntExistErr{}
		return
	}
	for {
		p = append([]string{existing.name}, p...)
		if !existing.parent.Valid {
			return
		}
		existing, ok = o.resources[existing.parent.Int64]
		if !ok {
			err = doesntExistErr{}
			return
		}
	}
}
