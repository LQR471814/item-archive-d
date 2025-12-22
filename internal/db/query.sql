-- name: CreateResource :one
insert into resource (parent_id, name, type, color, comments, image)
values (?, ?, ?, ?, ?, ?)
returning id;

-- name: UpdateResource :exec
update resource
set
	type = ?,
	color = ?,
	comments = ?,
	image = ?
where id = ?;

-- name: MoveResource :exec
update resource
set parent_id = ?
where id = ?;

-- name: ListResources :many
select * from resource
where parent_id is ?;

-- name: DeleteResource :exec
delete from resource
where id = ?;

-- name: GetResourceID :one
select id from resource
where parent_id is ? and name = ?;

-- name: MakeTrash :exec
insert into resource (id, name, type)
values (1, "__Trash__", "container");

