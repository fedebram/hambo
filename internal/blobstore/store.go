package blobstore

import (
	_ "crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/opencontainers/go-digest"
	ociv1 "github.com/opencontainers/image-spec/specs-go/v1"
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

func (s *Store) Put(expected ociv1.Descriptor, src io.Reader) (err error) {
	if err := expected.Digest.Validate(); err != nil {
		return fmt.Errorf("invalid blob digest: %w", err)
	}
	if expected.Size < 0 {
		return fmt.Errorf("invalid blob size: %d", expected.Size)
	}

	dir := filepath.Join(s.root, "blobs", expected.Digest.Algorithm().String())
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("creating blob directory: %w", err)
	}

	file, err := os.CreateTemp(filepath.Join(s.root, "staging"), "blob-")
	if err != nil {
		return fmt.Errorf("creating staging blob: %w", err)
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

	digester := expected.Digest.Algorithm().Digester()

	size, err := io.Copy(
		io.MultiWriter(file, digester.Hash()),
		src,
	)
	if err != nil {
		return fmt.Errorf("writing blob: %w", err)
	}

	if size != expected.Size {
		return fmt.Errorf(
			"blob size mismatch: expected %d, got %d",
			expected.Size, size,
		)
	}

	actual := digester.Digest()
	if actual != expected.Digest {
		return fmt.Errorf(
			"blob digest mismatch: expected %s, got %s",
			expected.Digest, actual,
		)
	}

	closeErr := file.Close()
	closed = true
	if closeErr != nil {
		return fmt.Errorf("closing blob: %w", closeErr)
	}

	path := filepath.Join(dir, expected.Digest.Encoded())
	if err := os.Rename(tempPath, path); err != nil {
		return fmt.Errorf("committing blob: %w", err)
	}

	return nil
}
