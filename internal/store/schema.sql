CREATE TABLE IF NOT EXISTS clients (
	public_key BLOB PRIMARY KEY,
	name       TEXT NOT NULL,
	role       TEXT NOT NULL,
	created_at INTEGER NOT NULL,
	-- only for workers, NULL for admins
	wg_public  BLOB,
	subnet     TEXT UNIQUE
) STRICT;

CREATE TABLE IF NOT EXISTS tokens (
	id         BLOB PRIMARY KEY,
	secret     BLOB NOT NULL,
	expires_at INTEGER NOT NULL
) STRICT;
