package store

import (
	"path/filepath"
	"testing"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()

	store, err := Open(filepath.Join(t.TempDir(), "hambo.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}

	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Errorf("close store: %v", err)
		}
	})

	return store
}

func newTestCollection[T any](t *testing.T) *Collection[T] {
	t.Helper()

	collection, err := NewCollection[T](newTestStore(t), "entries")
	if err != nil {
		t.Fatalf("create collection: %v", err)
	}

	return collection
}
