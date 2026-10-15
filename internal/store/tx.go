package store

import (
	"encoding/json"
	"fmt"

	bolt "go.etcd.io/bbolt"
)

func (s *Store) WriteTx(fn func(*bolt.Tx) error) error {
	err := s.db.Update(fn)
	if err != nil {
		return fmt.Errorf("write transaction: %w", err)
	}

	return nil
}

func (c *Collection[T]) CreateTx(tx *bolt.Tx, key string, value T) error {
	bucket := tx.Bucket(c.bucket)
	if bucket == nil {
		return fmt.Errorf("collection %q does not exist", c.bucket)
	}

	k := []byte(key)
	if bucket.Get(k) != nil {
		return fmt.Errorf("create entry %q in collection %q: %w", key, c.bucket, ErrAlreadyExists)
	}

	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("encode entry %q: %w", key, err)
	}
	if err := bucket.Put(k, data); err != nil {
		return fmt.Errorf("create entry %q in collection %q: %w", key, c.bucket, err)
	}

	return nil
}

func (c *Collection[T]) GetTx(tx *bolt.Tx, key string) (T, error) {
	var zero T

	bucket := tx.Bucket(c.bucket)
	if bucket == nil {
		return zero, fmt.Errorf("collection %q does not exist", c.bucket)
	}

	data := bucket.Get([]byte(key))
	if data == nil {
		return zero, fmt.Errorf("get entry %q from collection %q: %w", key, c.bucket, ErrNotFound)
	}

	var value T
	if err := json.Unmarshal(data, &value); err != nil {
		return zero, fmt.Errorf("decode entry %q from collection %q: %w", key, c.bucket, err)
	}

	return value, nil
}

func (c *Collection[T]) UpdateTx(tx *bolt.Tx, key string, value T) error {
	bucket := tx.Bucket(c.bucket)
	if bucket == nil {
		return fmt.Errorf("collection %q does not exist", c.bucket)
	}

	k := []byte(key)
	if bucket.Get(k) == nil {
		return fmt.Errorf("update entry %q in collection %q: %w", key, c.bucket, ErrNotFound)
	}

	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("encode entry %q: %w", key, err)
	}
	if err := bucket.Put(k, data); err != nil {
		return fmt.Errorf("update entry %q in collection %q: %w", key, c.bucket, err)
	}

	return nil
}

func (c *Collection[T]) ModifyTx(tx *bolt.Tx, key string, modify func(*T) error) error {
	value, err := c.GetTx(tx, key)
	if err != nil {
		return err
	}
	if err := modify(&value); err != nil {
		return fmt.Errorf("modify entry %q in collection %q: %w", key, c.bucket, err)
	}

	return c.UpdateTx(tx, key, value)
}

func (c *Collection[T]) DeleteTx(tx *bolt.Tx, key string) error {
	bucket := tx.Bucket(c.bucket)
	if bucket == nil {
		return fmt.Errorf("collection %q does not exist", c.bucket)
	}
	if err := bucket.Delete([]byte(key)); err != nil {
		return fmt.Errorf("delete entry %q from collection %q: %w", key, c.bucket, err)
	}

	return nil
}

func (c *Collection[T]) ListTx(tx *bolt.Tx) ([]Entry[T], error) {
	bucket := tx.Bucket(c.bucket)
	if bucket == nil {
		return nil, fmt.Errorf("collection %q does not exist", c.bucket)
	}

	entries := make([]Entry[T], 0)
	err := bucket.ForEach(func(key, data []byte) error {
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
	if err != nil {
		return nil, fmt.Errorf("list collection %q: %w", c.bucket, err)
	}

	return entries, nil
}
