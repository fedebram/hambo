//go:build integration

package integration

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	containerd "github.com/containerd/containerd/v2/client"
	"github.com/containerd/containerd/v2/pkg/cio"
	"github.com/containerd/containerd/v2/pkg/namespaces"
	"github.com/containerd/containerd/v2/pkg/oci"
	"github.com/containerd/errdefs"
	"github.com/fedebram/hambo/internal/taskutil"
)

const (
	sockPath = "/run/containerd/containerd.sock"

	testNamespace = "taskworker-test"

	// busybox lets us test scenarios like a task that refuses graceful shutdown.
	testImage = "docker.io/library/busybox:1.37"
)

var testClient *containerd.Client

// we need a shared containerd client for all integration tests
func TestMain(m *testing.M) {
	client, err := containerd.New(sockPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cannot reach containerd at %s: %v\n", sockPath, err)
		os.Exit(1)
	}
	testClient = client

	ctx := namespaces.WithNamespace(context.Background(), testNamespace)

	if _, err := client.GetImage(ctx, testImage); errdefs.IsNotFound(err) {
		if _, err := client.Pull(ctx, testImage, containerd.WithPullUnpack); err != nil {
			fmt.Fprintf(os.Stderr, "cannot pull %s: %v\n", testImage, err)
			_ = client.Close()
			os.Exit(1)
		}
	} else if err != nil {
		fmt.Fprintf(os.Stderr, "cannot get or pull image %s: %v\n", testImage, err)
		_ = client.Close()
		os.Exit(1)
	}

	code := m.Run()

	_ = client.Close()
	os.Exit(code)
}

type testEnv struct {
	t      *testing.T
	ctx    context.Context
	name   string
	tw     *taskutil.TaskWorker
	client *containerd.Client
}

func newTestEnv(t *testing.T) *testEnv {
	t.Helper()

	ctx := namespaces.WithNamespace(context.Background(), testNamespace)

	// container name must not have "/"
	// see containerd/pkg/identifiers/validate_test for examples
	// we create the container name from the name of the test, but subtests are separated with "/" so we replace that.
	name := "tw-" + strings.ReplaceAll(strings.ToLower(t.Name()), "/", "-")

	store, err := taskutil.NewTaskStore(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("NewTaskStore: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	e := &testEnv{
		t:      t,
		ctx:    ctx,
		name:   name,
		tw:     taskutil.NewTaskWorker(testClient, store, name),
		client: testClient,
	}
	t.Cleanup(e.forceCleanup)
	return e
}

func (e *testEnv) forceCleanup() {
	c, err := e.client.LoadContainer(e.ctx, e.name)
	if err != nil {
		return
	}
	if task, err := c.Task(e.ctx, nil); err == nil {
		_, _ = task.Delete(e.ctx, containerd.WithProcessKill)
	}
	_ = c.Delete(e.ctx, containerd.WithSnapshotCleanup)
}

// createContainer creates a new container with the provided test image.
// args are the process args. If nothing is passed the base config of the image is used.
func (e *testEnv) createContainer(args ...string) containerd.Container {
	e.t.Helper()

	img, err := e.client.GetImage(e.ctx, testImage)
	if err != nil {
		e.t.Fatalf("GetImage: %v", err)
	}

	specOpts := []oci.SpecOpts{oci.WithImageConfig(img)}
	if len(args) > 0 {
		specOpts = append(specOpts, oci.WithProcessArgs(args...))
	}

	c, err := e.client.NewContainer(e.ctx, e.name,
		containerd.WithImage(img),
		containerd.WithNewSnapshot(e.name+"-snapshot", img),
		containerd.WithNewSpec(specOpts...),
	)
	if err != nil {
		e.t.Fatalf("NewContainer: %v", err)
	}
	return c
}

func (e *testEnv) newTask(c containerd.Container) containerd.Task {
	e.t.Helper()
	task, err := c.NewTask(e.ctx, cio.NullIO)
	if err != nil {
		e.t.Fatalf("NewTask: %v", err)
	}
	return task
}

func (e *testEnv) start(task containerd.Task) {
	e.t.Helper()
	if err := task.Start(e.ctx); err != nil {
		e.t.Fatalf("task.Start: %v", err)
	}
}

// runningTask creates the task and starts it.
// args are the process args that are passed when creating the container.
func (e *testEnv) runningTask(args ...string) containerd.Task {
	e.t.Helper()
	task := e.newTask(e.createContainer(args...))
	e.start(task)
	return task
}

func (e *testEnv) status(task containerd.Task) containerd.ProcessStatus {
	e.t.Helper()
	st, err := task.Status(e.ctx)
	if err != nil {
		e.t.Fatalf("task.Status: %v", err)
	}
	return st.Status
}

func (e *testEnv) containerExists() bool {
	e.t.Helper()
	_, err := e.client.LoadContainer(e.ctx, e.name)
	if err == nil {
		return true
	}
	if errdefs.IsNotFound(err) {
		return false
	}
	e.t.Fatalf("LoadContainer: %v", err)
	return false
}

// waitStatus polls task state until the task reaches want, or fails after timeout.
func (e *testEnv) waitStatus(task containerd.Task, want containerd.ProcessStatus, timeout time.Duration) {
	e.t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		got := e.status(task)
		if got == want {
			return
		}
		if time.Now().After(deadline) {
			e.t.Fatalf("waiting for status %q: still %q after %s", want, got, timeout)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// waitExit blocks on the exit channel and returns the process exit code.
func (e *testEnv) waitExit(ch <-chan containerd.ExitStatus, timeout time.Duration) uint32 {
	e.t.Helper()
	select {
	case st := <-ch:
		code, _, err := st.Result()
		if err != nil {
			e.t.Fatalf("exit status: %v", err)
		}
		return code
	case <-time.After(timeout):
		e.t.Fatalf("timed out after %s waiting for task to exit", timeout)
		return 0
	}
}
