CREATE TABLE IF NOT EXISTS clients (
	public_key BLOB PRIMARY KEY,
	name       TEXT NOT NULL,
	role       TEXT NOT NULL,
	created_at INTEGER NOT NULL,
	-- only for workers, NULL for admins
	wg_public  BLOB,
	subnet     TEXT UNIQUE,
	-- NULL until the worker connects
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
	node_key   BLOB REFERENCES clients(public_key) ON DELETE CASCADE
) STRICT;

CREATE INDEX IF NOT EXISTS containers_node_key ON containers(node_key);
