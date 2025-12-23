-- name: CreateResource :one
insert into resource (parent_id, name, type, color, comments, image)
values (?, ?, ?, ?, ?, ?)
returning id;

-- name: UpdateResource :exec
update resource
set
	name = ?,
	type = ?,
	color = ?,
	comments = ?
where id = ?;

-- name: UpdateResourceImage :exec
update resource
set image = ?
where id = ?;

-- name: MoveResource :exec
update resource
set parent_id = ?
where id = ?;

-- name: ChangeParent :exec
update resource
set parent_id = @new_parent
where parent_id = @old_parent;

-- name: ListResources :many
select * from resource
where parent_id is ?;

-- name: GetResource :one
select * from resource
where id = ?;

-- name: DeleteResource :exec
delete from resource
where id = ?;

-- name: MakeTrash :exec
insert into resource (id, name, type)
values (1, "__Trash__", "container");

