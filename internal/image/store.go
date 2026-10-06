package image

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	ociv1 "github.com/opencontainers/image-spec/specs-go/v1"
	bolt "go.etcd.io/bbolt"
)

const imagesBucket = "images"

var ErrImageNotFound = errors.New("image not found")

type Image struct {
	Name       string           `json:"name"`
	Descriptor ociv1.Descriptor `json:"descriptor"`
}

type Store struct {
	db *bolt.DB
}

func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("create database directory: %w", err)
	}

	db, err := bolt.Open(path, 0o600, &bolt.Options{
		Timeout: 5 * time.Second,
	})
	if err != nil {
		return nil, fmt.Errorf("open bolt database: %w", err)
	}

	if err := db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(imagesBucket))
		return err
	}); err != nil {
		initErr := fmt.Errorf("initialize images bucket: %w", err)

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

func (s *Store) Save(image Image) error {
	if image.Name == "" {
		return errors.New("image name must not be empty")
	}

	value, err := json.Marshal(image)
	if err != nil {
		return fmt.Errorf("encode image %q: %w", image.Name, err)
	}

	if err := s.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(imagesBucket))
		if bucket == nil {
			return errors.New("images bucket does not exist")
		}

		return bucket.Put([]byte(image.Name), value)
	}); err != nil {
		return fmt.Errorf("save image %q: %w", image.Name, err)
	}

	return nil
}

func (s *Store) Get(name string) (Image, error) {
	var image Image

	if err := s.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(imagesBucket))
		if bucket == nil {
			return errors.New("images bucket does not exist")
		}

		value := bucket.Get([]byte(name))
		if value == nil {
			return ErrImageNotFound
		}

		if err := json.Unmarshal(value, &image); err != nil {
			return fmt.Errorf("decode image: %w", err)
		}

		return nil
	}); err != nil {
		return Image{}, fmt.Errorf("get image %q: %w", name, err)
	}

	return image, nil
}

func (s *Store) List() ([]Image, error) {
	images := make([]Image, 0)

	if err := s.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(imagesBucket))
		if bucket == nil {
			return errors.New("images bucket does not exist")
		}

		return bucket.ForEach(func(key, value []byte) error {
			if value == nil {
				return fmt.Errorf("unexpected nested bucket %q", key)
			}

			var image Image
			if err := json.Unmarshal(value, &image); err != nil {
				return fmt.Errorf("decode image %q: %w", key, err)
			}

			images = append(images, image)
			return nil
		})
	}); err != nil {
		return nil, fmt.Errorf("list images: %w", err)
	}

	return images, nil
}

func (s *Store) Delete(name string) error {
	if err := s.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(imagesBucket))
		if bucket == nil {
			return errors.New("images bucket does not exist")
		}

		if bucket.Get([]byte(name)) == nil {
			return ErrImageNotFound
		}

		return bucket.Delete([]byte(name))
	}); err != nil {
		return fmt.Errorf("delete image %q: %w", name, err)
	}

	return nil
}

func (s *Store) Close() error {
	if err := s.db.Close(); err != nil {
		return fmt.Errorf("close bolt database: %w", err)
	}

	return nil
}
