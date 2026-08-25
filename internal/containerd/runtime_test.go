//go:build integration

package containerd

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/containerd/containerd/v2/client"
	"github.com/containerd/errdefs"
	"github.com/fedebram/hambo/internal/container/containertest"
)

const (
	testImage          = "docker.io/library/nginx:alpine"
	testDifferentImage = "docker.io/library/redis:alpine"
)

func newTestRuntime(t *testing.T, options ...RuntimeOption) *Runtime {
	t.Helper()

	c, err := client.New(
		"/run/containerd/containerd.sock",
		client.WithDefaultNamespace("hambo-tests"),
	)
	if err != nil {
		t.Fatalf("unexpected create containerd client error: %v", err)
	}
	t.Cleanup(func() {
		_ = c.Close()
	})

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	if _, err := c.Version(ctx); err != nil {
		t.Fatalf("unexpected containerd version check error: %v", err)
	}

	return NewRuntime(c, options...)
}

func registerContainerCleanup(t *testing.T, runtime *Runtime, id string) {
	t.Helper()

	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		c, err := runtime.client.LoadContainer(ctx, id)
		if errdefs.IsNotFound(err) {
			return
		}
		if err != nil {
			t.Errorf("cleanup load container %q: %v", id, err)
			return
		}

		task, err := c.Task(ctx, nil)
		switch {
		case err == nil:
			if _, err := task.Delete(ctx, client.WithProcessKill); err != nil && !errdefs.IsNotFound(err) {
				t.Errorf("cleanup task for container %q: %v", id, err)
			}
		case errdefs.IsNotFound(err):
		default:
			t.Errorf("cleanup load task for container %q: %v", id, err)
		}

		if err := c.Delete(ctx, client.WithSnapshotCleanup); err != nil && !errdefs.IsNotFound(err) {
			t.Errorf("cleanup container %q: %v", id, err)
		}
	})
}

func TestContainerdConnection(t *testing.T) {
	_ = newTestRuntime(t)
}

func TestRuntimeContract(t *testing.T) {
	runtime := newTestRuntime(t)
	nextID := 0

	containertest.TestRuntime(
		t,
		runtime,
		testImage,
		testDifferentImage,
		func(t *testing.T) string {
			t.Helper()
			nextID++
			return fmt.Sprintf("hambo-test-%d", nextID)
		},
		func(t *testing.T, id string) {
			t.Helper()
			registerContainerCleanup(t, runtime, id)
		},
		time.Second,
	)
}
