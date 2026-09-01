package boltstore

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/fedebram/hambo/internal/container"
	bolt "go.etcd.io/bbolt"
)

const containersBucket = "containers"

type Store struct {
	db *bolt.DB
}

func Open(path string) (*Store, error) {
	db, err := bolt.Open(path, 0o600, nil)
	if err != nil {
		return nil, fmt.Errorf("open bolt database: %w", err)
	}

	if err := db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(containersBucket))
		return err
	}); err != nil {
		initErr := fmt.Errorf("initialize containers bucket: %w", err)

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

func (s *Store) Create(c container.Container) error {
	value, err := json.Marshal(c)
	if err != nil {
		return fmt.Errorf("encode container %q: %w", c.Name, err)
	}

	if err := s.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(containersBucket))
		if bucket == nil {
			return errors.New("containers bucket does not exist")
		}

		key := []byte(c.Name)
		if bucket.Get(key) != nil {
			return container.ErrAlreadyExists
		}

		return bucket.Put(key, value)
	}); err != nil {
		return fmt.Errorf("create container %q: %w", c.Name, err)
	}

	return nil
}

func (s *Store) Get(name string) (container.Container, error) {
	var value []byte

	if err := s.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(containersBucket))
		if bucket == nil {
			return errors.New("containers bucket does not exist")
		}

		raw := bucket.Get([]byte(name))
		if raw == nil {
			return container.ErrNotFound
		}

		value = bytes.Clone(raw)
		return nil
	}); err != nil {
		return container.Container{}, fmt.Errorf("get container %q: %w", name, err)
	}

	var c container.Container
	if err := json.Unmarshal(value, &c); err != nil {
		return container.Container{}, fmt.Errorf("decode container %q: %w", name, err)
	}

	return c, nil
}

func (s *Store) Modify(name string, modify func(*container.Container) error) error {
	if err := s.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(containersBucket))
		if bucket == nil {
			return errors.New("containers bucket does not exist")
		}

		key := []byte(name)
		value := bucket.Get(key)
		if value == nil {
			return container.ErrNotFound
		}

		var c container.Container
		if err := json.Unmarshal(value, &c); err != nil {
			return fmt.Errorf("decode container: %w", err)
		}

		if err := modify(&c); err != nil {
			return err
		}
		if c.Name != name {
			return fmt.Errorf(
				"container name cannot be changed: %w",
				container.ErrOperationNotAllowed,
			)
		}

		value, err := json.Marshal(c)
		if err != nil {
			return fmt.Errorf("encode container: %w", err)
		}

		return bucket.Put(key, value)
	}); err != nil {
		return fmt.Errorf("modify container %q: %w", name, err)
	}

	return nil
}

func (s *Store) Delete(name string) error {
	if err := s.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(containersBucket))
		if bucket == nil {
			return errors.New("containers bucket does not exist")
		}

		return bucket.Delete([]byte(name))
	}); err != nil {
		return fmt.Errorf("delete container %q: %w", name, err)
	}

	return nil
}

func (s *Store) List() ([]container.Container, error) {
	containers := make([]container.Container, 0)

	if err := s.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(containersBucket))
		if bucket == nil {
			return errors.New("containers bucket does not exist")
		}

		return bucket.ForEach(func(key, value []byte) error {
			if value == nil {
				return fmt.Errorf("unexpected nested bucket %q", key)
			}

			var c container.Container
			// I wonder if it is ok to unmarshal inside the transaction...
			// copy the bytes and unmarshal outside?
			if err := json.Unmarshal(value, &c); err != nil {
				return fmt.Errorf("decode container %q: %w", key, err)
			}

			containers = append(containers, c)
			return nil
		})
	}); err != nil {
		return nil, fmt.Errorf("list containers: %w", err)
	}

	return containers, nil
}

func (s *Store) Close() error {
	if err := s.db.Close(); err != nil {
		return fmt.Errorf("close bolt database: %w", err)
	}
	return nil
}
