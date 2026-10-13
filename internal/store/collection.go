package store

import (
	"encoding/json"
	"fmt"

	bolt "go.etcd.io/bbolt"
)

// TODO: we need to create a registry of name -> collection type
// in memory and a schema like thing saved on bolt. Because if someone creates
// a collection with the same name but a different type, then we fuck up everything.

type Collection[T any] struct {
	db     *bolt.DB
	bucket []byte
}

type Entry[T any] struct {
	Key   string
	Value T
}

func NewCollection[T any](s *Store, name string) (*Collection[T], error) {
	bucket := []byte(name)
	if err := s.db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists(bucket)
		return err
	}); err != nil {
		return nil, fmt.Errorf("initialize collection %q: %w", name, err)
	}

	return &Collection[T]{db: s.db, bucket: bucket}, nil
}

func (c *Collection[T]) Create(key string, value T) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("encode entry %q: %w", key, err)
	}

	if err := c.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(c.bucket)
		if bucket == nil {
			return fmt.Errorf("collection %q does not exist", c.bucket)
		}

		k := []byte(key)
		if bucket.Get(k) != nil {
			return ErrAlreadyExists
		}
		return bucket.Put(k, data)
	}); err != nil {
		return fmt.Errorf("create entry %q in collection %q: %w", key, c.bucket, err)
	}

	return nil
}

func (c *Collection[T]) Get(key string) (T, error) {
	var value T

	if err := c.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(c.bucket)
		if bucket == nil {
			return fmt.Errorf("collection %q does not exist", c.bucket)
		}

		data := bucket.Get([]byte(key))
		if data == nil {
			return ErrNotFound
		}
		if err := json.Unmarshal(data, &value); err != nil {
			return fmt.Errorf("decode entry: %w", err)
		}
		return nil
	}); err != nil {
		var zero T
		return zero, fmt.Errorf("get entry %q from collection %q: %w", key, c.bucket, err)
	}

	return value, nil
}

func (c *Collection[T]) Modify(key string, modify func(*T) error) error {
	if err := c.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(c.bucket)
		if bucket == nil {
			return fmt.Errorf("collection %q does not exist", c.bucket)
		}

		k := []byte(key)
		data := bucket.Get(k)
		if data == nil {
			return ErrNotFound
		}

		var value T
		if err := json.Unmarshal(data, &value); err != nil {
			return fmt.Errorf("decode entry: %w", err)
		}
		if err := modify(&value); err != nil {
			return err
		}

		data, err := json.Marshal(value)
		if err != nil {
			return fmt.Errorf("encode entry: %w", err)
		}
		return bucket.Put(k, data)
	}); err != nil {
		return fmt.Errorf("modify entry %q in collection %q: %w", key, c.bucket, err)
	}

	return nil
}

func (c *Collection[T]) Delete(key string) error {
	if err := c.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(c.bucket)
		if bucket == nil {
			return fmt.Errorf("collection %q does not exist", c.bucket)
		}
		return bucket.Delete([]byte(key))
	}); err != nil {
		return fmt.Errorf("delete entry %q from collection %q: %w", key, c.bucket, err)
	}

	return nil
}

func (c *Collection[T]) List() ([]Entry[T], error) {
	entries := make([]Entry[T], 0)

	if err := c.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(c.bucket)
		if bucket == nil {
			return fmt.Errorf("collection %q does not exist", c.bucket)
		}

		return bucket.ForEach(func(key, data []byte) error {
			if data == nil {
				return fmt.Errorf("unexpected nested bucket %q", key)
			}

			var value T
			if err := json.Unmarshal(data, &value); err != nil {
				return fmt.Errorf("decode entry %q: %w", key, err)
			}
			entries = append(entries, Entry[T]{Key: string(key), Value: value})
			return nil
		})
	}); err != nil {
		return nil, fmt.Errorf("list collection %q: %w", c.bucket, err)
	}

	return entries, nil
}
