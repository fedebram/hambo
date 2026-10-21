package sqlitestore

import (
	"database/sql"
	"errors"

	_ "github.com/mattn/go-sqlite3"
)

type Store struct {
	ReadDB  *sql.DB
	WriteDB *sql.DB
}

func Open(path string) (*Store, error) {
	writePath := path + "?_journal_mode=WAL" +
		"&_foreign_keys=on&_busy_timeout=5000" +
		"&_txlock=immediate&_synchronous=FULL"
	writeDB, err := sql.Open("sqlite3", writePath)
	if err != nil {
		return nil, err
	}
	writeDB.SetMaxOpenConns(1)
	writeDB.SetMaxIdleConns(1)

	if err := writeDB.Ping(); err != nil {
		writeDB.Close()
		return nil, err
	}

	readPath := path + "?_foreign_keys=on" +
		"&_busy_timeout=5000&_query_only=on"
	readDB, err := sql.Open("sqlite3", readPath)
	if err != nil {
		writeDB.Close()
		return nil, err
	}
	readDB.SetMaxOpenConns(4)
	readDB.SetMaxIdleConns(4)

	if err := readDB.Ping(); err != nil {
		readDB.Close()
		writeDB.Close()
		return nil, err
	}

	return &Store{
		ReadDB:  readDB,
		WriteDB: writeDB,
	}, nil
}

func (s *Store) Close() error {
	return errors.Join(
		s.ReadDB.Close(),
		s.WriteDB.Close(),
	)
}

func (s *Store) CreateTables() error {
	_, err := s.WriteDB.Exec(`
		CREATE TABLE IF NOT EXISTS nodes (
			id                TEXT NOT NULL PRIMARY KEY,
			name              TEXT NOT NULL,
			runtime_ready     INTEGER NOT NULL DEFAULT 0
			                  CHECK (runtime_ready IN (0, 1)),
			last_seen_at      TEXT,
			status_session_id TEXT NOT NULL DEFAULT '',
			status_sequence   INTEGER NOT NULL DEFAULT 0
			                  CHECK (status_sequence >= 0)
		) STRICT;
	`)
	return err
}
