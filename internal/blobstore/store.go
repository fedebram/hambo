package blobstore

import (
	_ "crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/opencontainers/go-digest"
)

type Store struct {
	root string
}

func New(root string) (*Store, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolving blob store path: %w", err)
	}

	for _, dir := range []string{"blobs", "staging"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o700); err != nil {
			return nil, fmt.Errorf("creating blob store %s directory: %w", dir, err)
		}
	}

	return &Store{root: root}, nil
}

func (s *Store) PutBytes(data []byte) (_ digest.Digest, err error) {
	d := digest.FromBytes(data)

	dir := filepath.Join(s.root, "blobs", d.Algorithm().String())
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("creating blob directory: %w", err)
	}

	file, err := os.CreateTemp(filepath.Join(s.root, "staging"), "blob-")
	if err != nil {
		return "", fmt.Errorf("creating staging blob: %w", err)
	}

	tempPath := file.Name()
	closed := false
	defer func() {
		if !closed {
			if closeErr := file.Close(); closeErr != nil {
				err = errors.Join(err, fmt.Errorf("closing staging blob: %w", closeErr))
			}
		}

		if removeErr := os.Remove(tempPath); removeErr != nil &&
			!errors.Is(removeErr, os.ErrNotExist) {
			err = errors.Join(err, fmt.Errorf("removing staging blob: %w", removeErr))
		}
	}()

	if _, err := file.Write(data); err != nil {
		return "", fmt.Errorf("writing blob: %w", err)
	}

	closeErr := file.Close()
	closed = true
	if closeErr != nil {
		return "", fmt.Errorf("closing blob: %w", closeErr)
	}

	path := filepath.Join(dir, d.Encoded())
	if err := os.Rename(tempPath, path); err != nil {
		return "", fmt.Errorf("committing blob: %w", err)
	}

	return d, nil
}
