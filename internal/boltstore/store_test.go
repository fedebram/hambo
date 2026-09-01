package boltstore

import (
	"path/filepath"
	"testing"

	"github.com/fedebram/hambo/internal/container"
	"github.com/fedebram/hambo/internal/container/containertest"
)

func TestStore(t *testing.T) {
	containertest.TestStore(t, func(t *testing.T) container.Store {
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
	})
}
