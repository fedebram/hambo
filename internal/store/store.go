package store

import (
	"errors"
	"fmt"

	bolt "go.etcd.io/bbolt"
)

type Store struct {
	db *bolt.DB
}

var ErrAlreadyExists = errors.New("already exists")
var ErrNotFound = errors.New("not found")

func Open(path string) (*Store, error) {
	db, err := bolt.Open(path, 0o600, nil)
	if err != nil {
		return nil, fmt.Errorf("open bolt database: %w", err)
	}

	return &Store{db: db}, nil
}

func (s *Store) Close() error {
	if err := s.db.Close(); err != nil {
		return fmt.Errorf("close bolt database: %w", err)
	}

	return nil
}
