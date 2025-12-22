create table resource (
	id integer primary key autoincrement,
	parent_id integer references resource(id)
		on update cascade
		on delete cascade,

	name text not null,
	type text not null,
	color text not null,
	comments text not null,
	image integer
);

create virtual table resource_fts using fts5(
	name,
	color,
	comments,
	content=resource,
	content_rowid=id,
	tokenize='trigram'
);

create trigger resource_ai after insert on resource begin
	insert into resource_fts(rowid, name, color, comments)
	values (new.id, new.name, new.color, new.comments);
end;
create trigger resource_ad after delete on resource begin
	insert into resource_fts(resource_fts, rowid, name, color, comments)
	values ('delete', old.id, old.name, old.color, old.comments);
end;
create trigger resource_au after update on resource begin
	insert into resource_fts(resource_fts, rowid, name, color, comments)
	values ('delete', old.id, old.name, old.color, old.comments);
	insert into resource_fts(rowid, name, color, comments)
	values (new.id, new.name, new.color, new.comments);
end;

