package store

import (
	"database/sql"
	_ "embed"
	"errors"

	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

//go:embed schema.sql
var schema string

// ErrConflict means a row with the same unique value already exists.
var ErrConflict = errors.New("already exists")

type Store struct {
	write *sql.DB
	read  *sql.DB
}

// Open opens the database at path.
func Open(path string) (*Store, error) {
	write, err := sql.Open("sqlite", "file:"+path+
		"?_pragma=journal_mode(WAL)"+
		"&_pragma=foreign_keys(1)"+
		"&_pragma=busy_timeout(5000)"+
		"&_pragma=synchronous(FULL)"+
		"&_txlock=immediate")
	if err != nil {
		return nil, err
	}
	write.SetMaxOpenConns(1)
	// the first connection creates the file and turns on WAL
	if err := write.Ping(); err != nil {
		write.Close()
		return nil, err
	}
	if _, err := write.Exec(schema); err != nil {
		write.Close()
		return nil, err
	}

	read, err := sql.Open("sqlite", "file:"+path+
		"?_pragma=foreign_keys(1)"+
		"&_pragma=busy_timeout(5000)"+
		"&_pragma=query_only(1)")
	if err != nil {
		write.Close()
		return nil, err
	}
	read.SetMaxOpenConns(4)

	return &Store{write: write, read: read}, nil
}

func (s *Store) Close() error {
	return errors.Join(s.read.Close(), s.write.Close())
}

// isConflict reports whether err is a UNIQUE or PRIMARY KEY failure.
func isConflict(err error) bool {
	var sqliteErr *sqlite.Error
	if !errors.As(err, &sqliteErr) {
		return false
	}
	code := sqliteErr.Code()
	return code == sqlite3.SQLITE_CONSTRAINT_UNIQUE || code == sqlite3.SQLITE_CONSTRAINT_PRIMARYKEY
}
