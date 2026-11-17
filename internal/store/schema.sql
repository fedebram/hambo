CREATE TABLE IF NOT EXISTS admins (
	public_key BLOB PRIMARY KEY,
	name       TEXT NOT NULL UNIQUE,
	created_at INTEGER NOT NULL
) STRICT;

CREATE TABLE IF NOT EXISTS nodes (
	public_key BLOB PRIMARY KEY,
	name       TEXT NOT NULL UNIQUE,
	created_at INTEGER NOT NULL,
	wg_public  BLOB NOT NULL,
	subnet     TEXT NOT NULL UNIQUE,
	-- NULL until the node first connects
	endpoint   TEXT
) STRICT;

CREATE TABLE IF NOT EXISTS tokens (
	id         BLOB PRIMARY KEY,
	secret     BLOB NOT NULL,
	expires_at INTEGER NOT NULL
) STRICT;

CREATE TABLE IF NOT EXISTS containers (
	id         TEXT PRIMARY KEY,
	name       TEXT NOT NULL UNIQUE,
	spec       BLOB NOT NULL,
	created_at INTEGER NOT NULL,
	-- the node it runs on, NULL while pending.
	-- removing the node removes its containers.
	node_key   BLOB REFERENCES nodes(public_key) ON DELETE CASCADE
) STRICT;

CREATE INDEX IF NOT EXISTS containers_node_key ON containers(node_key);
