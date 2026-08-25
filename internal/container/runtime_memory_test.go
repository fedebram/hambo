package container_test

import (
	"fmt"
	"testing"

	"github.com/fedebram/hambo/internal/container"
	"github.com/fedebram/hambo/internal/container/containertest"
)

func TestMemoryRuntime(t *testing.T) {
	runtime := container.NewMemoryRuntime()
	nextID := 0

	containertest.TestRuntime(
		t,
		runtime,
		"alpine",
		"redis",
		func(t *testing.T) string {
			t.Helper()
			nextID++
			return fmt.Sprintf("hambo-test-%d", nextID)
		},
		func(*testing.T, string) {},
		0,
	)
}
