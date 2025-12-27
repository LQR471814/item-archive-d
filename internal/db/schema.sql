PRAGMA foreign_keys = ON;

create table resource (
	id integer primary key autoincrement,
	parent_id integer,

	name text not null,
	type text not null,
	comments text not null,
	image integer,

	foreign key(parent_id) references resource(id)
		on update cascade
		on delete cascade
);

create virtual table resource_fts using fts5(
	name,
	comments,
	content=resource,
	content_rowid=id,
	tokenize='trigram'
);

create trigger resource_ai after insert on resource begin
	insert into resource_fts(rowid, name, comments)
	values (new.id, new.name, new.comments);
end;
create trigger resource_ad after delete on resource begin
	insert into resource_fts(resource_fts, rowid, name, comments)
	values ('delete', old.id, old.name, old.comments);
end;
create trigger resource_au after update on resource begin
	insert into resource_fts(resource_fts, rowid, name, comments)
	values ('delete', old.id, old.name, old.comments);
	insert into resource_fts(rowid, name, comments)
	values (new.id, new.name, new.comments);
end;

