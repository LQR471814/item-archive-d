package db

import (
	"database/sql"
	"errors"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
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

type fkeyErr struct{}

func (fkeyErr) Error() string {
	return "foreign key constraint violated"
}

type oracle struct {
	resources map[int64]Resource
	count     *int64
}

func newOracle() oracle {
	count := int64(1)
	return oracle{
		resources: make(map[int64]Resource),
		count:     &count,
	}
}

func (o oracle) createResource(r Resource) (err error) {
	if r.ID == 0 {
		r.ID = *o.count
		defer func() {
			if err == nil {
				*o.count++
			}
		}()
	}
	_, ok := o.resources[r.ID]
	if ok {
		err = alreadyExistsErr{}
		return
	}
	if r.ParentID.Valid {
		_, ok := o.resources[r.ParentID.Int64]
		if !ok && r.ParentID.Int64 != r.ID {
			err = fkeyErr{}
			return
		}
	}
	o.resources[r.ID] = r
	return
}

func (o oracle) updateResource(r UpdateResourceParams) []int64 {
	existing, ok := o.resources[r.ID]
	if !ok {
		return nil
	}
	existing.Name = r.Name
	existing.Comments = r.Comments
	existing.Type = r.Type
	o.resources[r.ID] = existing
	return []int64{r.ID}
}

func (o oracle) updateResourceImage(r UpdateResourceImageParams) []int64 {
	existing, ok := o.resources[r.ID]
	if !ok {
		return nil
	}
	existing.Image = r.Image
	o.resources[r.ID] = existing
	return []int64{r.ID}
}

func (o oracle) moveResources(params MoveResourcesParams) (updated []int64, err error) {
	if len(params.Ids) == 0 {
		return
	}
	values := make(map[int64]struct{})
	for _, id := range params.Ids {
		values[id] = struct{}{}
	}
	for id := range values {
		existing, ok := o.resources[id]
		if !ok {
			continue
		}
		if params.NewParent.Valid {
			_, ok = o.resources[params.NewParent.Int64]
			if !ok {
				err = fkeyErr{}
				return
			}
		}
		existing.ParentID = params.NewParent
		o.resources[id] = existing
		updated = append(updated, id)
	}
	return
}

func (o oracle) changeParent(params ChangeParentParams) (err error) {
	for _, r := range o.resources {
		if r.ParentID == params.OldParent {
			if params.NewParent.Valid {
				_, ok := o.resources[params.NewParent.Int64]
				if !ok {
					err = fkeyErr{}
					return
				}
			}
			r.ParentID = params.NewParent
			o.resources[r.ID] = r
		}
	}
	return
}

func (o oracle) listResources(parentId sql.NullInt64) (out []Resource) {
	for _, r := range o.resources {
		if r.ParentID == parentId {
			out = append(out, r)
		}
	}
	return
}

func (o oracle) getResource(id int64) (Resource, error) {
	existing, ok := o.resources[id]
	if !ok {
		return Resource{}, doesntExistErr{}
	}
	return existing, nil
}

func (o oracle) deleteResource(id int64) {
	if id == 0 {
		panic("invalid id, cannot delete 0")
	}
	delete(o.resources, id)
	for _, r := range o.resources {
		if r.ParentID.Int64 == id {
			o.deleteResource(r.ID)
		}
	}
}

func (o oracle) resolve(path string) (found sql.NullInt64, err error) {
	var current Resource
path:
	for name := range strings.SplitSeq(path, "/") {
		if name == "" {
			continue
		}
		for _, r := range o.resources {
			if r.ParentID.Int64 == current.ID && r.Name == name {
				current = r
				continue path
			}
		}
		err = doesntExistErr{}
		return
	}
	if current.ID == 0 {
		return
	}
	found = sql.NullInt64{Valid: true, Int64: current.ID}
	return
}

func (o oracle) getPath(id int64) (p []string, err error) {
	existing, ok := o.resources[id]
	if !ok {
		err = doesntExistErr{}
		return
	}
	for {
		p = append([]string{existing.Name}, p...)
		if !existing.ParentID.Valid {
			return
		}
		existing, ok = o.resources[existing.ParentID.Int64]
		if !ok {
			err = doesntExistErr{}
			return
		}
	}
}

func TestDB(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// ensure db is cleared
		os.Remove("testing.db")
		os.Remove("testing.db-wal")
		os.Remove("testing.db-shm")
		_, err := os.Stat("testing.db")
		if err == nil {
			t.Fatal("failed to remove testing.db")
		}
		_, err = os.Stat("testing.db-wal")
		if err == nil {
			t.Fatal("failed to remove testing.db-wal")
		}
		_, err = os.Stat("testing.db-shm")
		if err == nil {
			t.Fatal("failed to remove testing.db-shm")
		}

		_, qry, err := Open(t.Context(), "testing.db", "")
		if err != nil {
			t.Fatal(err)
		}
		model := newOracle()

		// assertResourceStateEqual checks if the all rows in the real DB and
		// the rows in the model are exactly the same
		assertResourceStateEqual := func(t *rapid.T) {
			resourcesReal, err := qry.ListResources(t.Context(), sql.NullInt64{})
			if err != nil {
				t.Fatal("(real) unexpected error:", err)
			}
			resourcesModel := model.listResources(sql.NullInt64{})
			slices.SortFunc(resourcesReal, func(a, b Resource) int {
				return int(a.ID - b.ID)
			})
			slices.SortFunc(resourcesModel, func(a, b Resource) int {
				return int(a.ID - b.ID)
			})
			require.Equal(t, resourcesModel, resourcesReal)
		}

		types := []string{"container", "item"}
		names := []string{}
		t.Repeat(map[string]func(*rapid.T){
			"CreateResource": func(t *rapid.T) {
				parentIdValue := rapid.Int64Min(0).Draw(t, "parentId")
				parentId := sql.NullInt64{Int64: parentIdValue, Valid: true}
				if parentIdValue == 0 {
					parentId.Valid = false
				}
				name := rapid.String().Draw(t, "name")
				typeStr := rapid.SampledFrom(types).Draw(t, "type")
				comments := rapid.String().Draw(t, "comments")
				imageValue := rapid.Int64Min(0).Draw(t, "image")
				image := sql.NullInt64{Int64: imageValue, Valid: true}
				if imageValue == 0 {
					image.Valid = false
				}

				_, errReal := qry.CreateResource(t.Context(), CreateResourceParams{
					ParentID: parentId,
					Name:     name,
					Type:     typeStr,
					Image:    image,
					Comments: comments,
				})
				errModel := model.createResource(Resource{
					ParentID: parentId,
					Name:     name,
					Type:     typeStr,
					Image:    image,
					Comments: comments,
				})
				if errModel != nil {
					if errors.Is(errModel, alreadyExistsErr{}) {
						require.ErrorContains(t, errReal, "conflict")
						return
					}
					if errors.Is(errModel, fkeyErr{}) {
						require.ErrorContains(t, errReal, "FOREIGN KEY")
						return
					}
				}
				if errReal != nil {
					t.Fatal("(real) unexpected error:", errReal)
				}
				names = append(names, name)

				assertResourceStateEqual(t)
			},
			"UpdateResourceMetadata": func(t *rapid.T) {
				id := rapid.Int64Min(1).Draw(t, "id")
				name := rapid.String().Draw(t, "name")
				comments := rapid.String().Draw(t, "comments")
				typeStr := rapid.SampledFrom(types).Draw(t, "type")

				params := UpdateResourceParams{
					ID:       id,
					Name:     name,
					Type:     typeStr,
					Comments: comments,
				}
				updatedReal, err := qry.UpdateResource(t.Context(), params)
				updatedModel := model.updateResource(params)
				if err != nil {
					t.Fatal("(real) unexpected error:", err)
				}
				slices.Sort(updatedModel)
				slices.Sort(updatedReal)
				require.Equal(t, updatedModel, updatedReal)

				if len(updatedModel) == 0 {
					return
				}

				realResource, err := qry.GetResource(t.Context(), id)
				if err != nil {
					t.Fatal("(real) unexpected error:", err)
				}
				modelResource, err := model.getResource(id)
				if err != nil {
					t.Fatal("(model) unexpected error:", err)
				}

				require.Equal(t, modelResource, realResource)
			},
			"UpdateResourceImage": func(t *rapid.T) {
				id := rapid.Int64Min(1).Draw(t, "id")
				imageValue := rapid.Int64().Draw(t, "image")
				image := sql.NullInt64{Int64: imageValue, Valid: true}
				if imageValue == 0 {
					image.Valid = false
				}

				params := UpdateResourceImageParams{
					ID:    id,
					Image: image,
				}
				updatedReal, err := qry.UpdateResourceImage(t.Context(), params)
				updatedModel := model.updateResourceImage(params)
				if err != nil {
					t.Fatal("(real) unexpected error:", err)
				}
				slices.Sort(updatedModel)
				slices.Sort(updatedReal)
				require.Equal(t, updatedModel, updatedReal)

				if len(updatedModel) == 0 {
					return
				}

				realResource, err := qry.GetResource(t.Context(), id)
				if err != nil {
					t.Fatal("(real) unexpected error:", err)
				}
				modelResource, err := model.getResource(id)
				if err != nil {
					t.Fatal("(model) unexpected error:", err)
				}

				require.Equal(t, modelResource, realResource)
			},
			"MoveResources": func(t *rapid.T) {
				ids := rapid.SliceOf(rapid.Int64Min(1)).Draw(t, "ids")
				parentIDValue := rapid.Int64Min(0).Draw(t, "parentID")
				parentID := sql.NullInt64{Valid: true, Int64: parentIDValue}
				if parentIDValue == 0 {
					parentID.Valid = false
				}

				params := MoveResourcesParams{
					Ids:       ids,
					NewParent: parentID,
				}
				updatedReal, errReal := qry.MoveResources(t.Context(), params)
				updatedModel, errModel := model.moveResources(params)
				if errModel != nil {
					require.ErrorContains(t, errReal, "FOREIGN KEY", "model error: %v", errModel)
					return
				}
				if errReal != nil {
					t.Fatal("(real) unexpected error:", err)
				}
				slices.Sort(updatedReal)
				slices.Sort(updatedModel)
				require.Equal(t, updatedModel, updatedReal)

				assertResourceStateEqual(t)
			},
			"ChangeParent": func(t *rapid.T) {
				parentIDValue := rapid.Int64Min(0).Draw(t, "parentID")
				parentID := sql.NullInt64{Valid: true, Int64: parentIDValue}
				if parentIDValue == 0 {
					parentID.Valid = false
				}
				newParentIDValue := rapid.Int64Min(0).Draw(t, "parentID")
				newParentID := sql.NullInt64{Valid: true, Int64: newParentIDValue}
				if newParentIDValue == 0 {
					newParentID.Valid = false
				}

				params := ChangeParentParams{
					OldParent: parentID,
					NewParent: newParentID,
				}
				errReal := qry.ChangeParent(t.Context(), params)
				errModel := model.changeParent(params)
				if errModel != nil {
					require.ErrorContains(t, errReal, "FOREIGN KEY", "model error: %v", errModel)
					return
				}
				if errReal != nil {
					t.Fatal("(real) unexpected error:", errReal)
				}

				assertResourceStateEqual(t)
			},
			"DeleteResource": func(t *rapid.T) {
				id := rapid.Int64Min(1).Draw(t, "id")
				err := qry.DeleteResource(t.Context(), id)
				if err != nil {
					t.Fatal("(real) unexpected error:", err)
				}
				model.deleteResource(id)
				assertResourceStateEqual(t)
			},
			"Resolve": func(t *rapid.T) {
				if len(names) == 0 {
					return
				}
				path := rapid.SliceOf(rapid.SampledFrom(names)).Draw(t, "path")
				p := strings.Join(path, "/")

				foundReal, errReal := qry.Resolve(t.Context(), p)
				foundModel, errModel := model.resolve(p)
				if errModel != nil {
					require.ErrorIs(t, errReal, sql.ErrNoRows, "model error: %v", errModel)
					return
				}
				if errReal != nil {
					t.Fatal("(real) unexpected error:", errReal)
				}

				require.Equal(t, foundModel, foundReal)
				assertResourceStateEqual(t)
			},
			"GetPath": func(t *rapid.T) {
				id := rapid.Int64Min(1).Draw(t, "id")

				pathReal, errReal := qry.GetPath(t.Context(), id)
				pathModel, errModel := model.getPath(id)
				if len(pathReal) == len(pathModel) && len(pathReal) == 0 {
					assertResourceStateEqual(t)
					return
				}
				if errModel != nil {
					require.ErrorIs(t, errReal, sql.ErrNoRows, "model error: %v", errModel)
					return
				}
				if errReal != nil {
					t.Fatal("(real) unexpected error:", errReal)
				}

				require.Equal(t, pathModel, pathReal)
				assertResourceStateEqual(t)
			},
		})
	})
}
