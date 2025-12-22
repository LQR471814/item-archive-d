create table resource (
	id integer primary key autoincrement,
	parent_id integer references resource(id)
		on update cascade
		on delete cascade,

	name text not null,
	type text,
	color text,
	comments text,
	image integer
);

