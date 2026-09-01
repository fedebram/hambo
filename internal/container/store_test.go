package container_test

import (
	"testing"

	"github.com/fedebram/hambo/internal/container"
	"github.com/fedebram/hambo/internal/container/containertest"
)

func TestMemoryStore(t *testing.T) {
	containertest.TestStore(t, func(*testing.T) container.Store {
		return container.NewMemoryStore()
	})
}
