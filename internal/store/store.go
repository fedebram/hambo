package store

import (
	"errors"
	"fmt"

	bolt "go.etcd.io/bbolt"
)

const entriesBucket = "entries"

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

	if err := db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(entriesBucket))
		return err
	}); err != nil {
		initErr := fmt.Errorf("initialize entries bucket: %w", err)

		if closeErr := db.Close(); closeErr != nil {
			return nil, errors.Join(
				initErr,
				fmt.Errorf("close bolt database: %w", closeErr),
			)
		}

		return nil, initErr
	}

	return &Store{db: db}, nil
}

func (s *Store) Create(key, value string) error {
	if err := s.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(entriesBucket))
		if bucket == nil {
			return errors.New("entries bucket does not exist")
		}

		k := []byte(key)
		if bucket.Get(k) != nil {
			return ErrAlreadyExists
		}

		return bucket.Put(k, []byte(value))
	}); err != nil {
		return fmt.Errorf("create entry %q: %w", key, err)
	}

	return nil
}

func (s *Store) Get(key string) (string, error) {
	var value string

	if err := s.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(entriesBucket))
		if bucket == nil {
			return errors.New("entries bucket does not exist")
		}

		raw := bucket.Get([]byte(key))
		if raw == nil {
			return ErrNotFound
		}

		value = string(raw)
		return nil
	}); err != nil {
		return "", fmt.Errorf("get entry %q: %w", key, err)
	}

	return value, nil
}

func (s *Store) Modify(key string, modify func(string) (string, error)) error {
	if err := s.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(entriesBucket))
		if bucket == nil {
			return errors.New("entries bucket does not exist")
		}

		raw := bucket.Get([]byte(key))
		if raw == nil {
			return ErrNotFound
		}

		value, err := modify(string(raw))
		if err != nil {
			return err
		}

		return bucket.Put([]byte(key), []byte(value))
	}); err != nil {
		return fmt.Errorf("modify entry %q: %w", key, err)
	}

	return nil
}

func (s *Store) Delete(key string) error {
	if err := s.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(entriesBucket))
		if bucket == nil {
			return errors.New("entries bucket does not exist")
		}

		return bucket.Delete([]byte(key))
	}); err != nil {
		return fmt.Errorf("delete entry %q: %w", key, err)
	}

	return nil
}

type Entry struct {
	Key   string
	Value string
}

func (s *Store) List() ([]Entry, error) {
	entries := make([]Entry, 0)

	if err := s.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(entriesBucket))
		if bucket == nil {
			return errors.New("entries bucket does not exist")
		}

		return bucket.ForEach(func(key, value []byte) error {
			if value == nil {
				return fmt.Errorf("unexpected nested bucket %q", key)
			}

			entries = append(entries, Entry{
				Key:   string(key),
				Value: string(value),
			})
			return nil
		})
	}); err != nil {
		return nil, fmt.Errorf("list entries: %w", err)
	}

	return entries, nil
}

func (s *Store) Close() error {
	if err := s.db.Close(); err != nil {
		return fmt.Errorf("close bolt database: %w", err)
	}
	return nil
}
