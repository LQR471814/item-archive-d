-- name: CreateResource :one
insert into resource (parent_id, name, type, comments, image)
values (?, ?, ?, ?, ?)
returning id;

-- name: UpdateResource :many
update resource
set
	name = ?,
	type = ?,
	comments = ?
where id = ?
returning id;

-- name: UpdateResourceImage :many
update resource
set image = ?
where id = ?
returning id;

-- name: ChangeParent :exec
update resource
set parent_id = @new_parent
where parent_id is @old_parent;

-- name: MoveResources :many
update resource
set parent_id = @new_parent
where id in (sqlc.slice('ids'))
returning id;

-- name: ListResources :many
select * from resource
where parent_id is ?;

-- name: GetResource :one
select * from resource
where id = ?;

-- name: DeleteResource :exec
delete from resource
where id = ?;

