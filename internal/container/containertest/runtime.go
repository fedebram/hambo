// Package containertest provides reusable tests for container contracts.
package containertest

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/fedebram/hambo/internal/container"
)

// Contract tests. Inspired by https://quii.gitbook.io/learn-go-with-tests/testing-fundamentals/working-without-mocks
// The Memory Runtime and Containerd Runtime implementation need to adhere to these tests.

func TestRuntime(
	t *testing.T,
	runtime container.Runtime,
	image string,
	differentImage string,
	newContainerID func(*testing.T) string,
	registerCleanup func(*testing.T, string),
	startDelay time.Duration,
) {
	t.Helper()
	if differentImage == image {
		t.Fatal("runtime contract requires two different container images")
	}
	// we need to wait before stopping the task because the container might not
	// be ready to handle the stop signal yet.
	waitAfterStart := func() {
		if startDelay > 0 {
			time.Sleep(startDelay)
		}
	}

	t.Run("inspect missing container returns not found", func(t *testing.T) {
		id := newContainerID(t)
		registerCleanup(t, id)

		_, err := runtime.Inspect(t.Context(), id)
		if !errors.Is(err, container.ErrNotFound) {
			t.Fatalf("got error %v, want %v", err, container.ErrNotFound)
		}
	})

	t.Run("created container can be inspected", func(t *testing.T) {
		id := newContainerID(t)
		registerCleanup(t, id)

		if err := runtime.CreateContainer(t.Context(), id, image); err != nil {
			t.Fatalf("unexpected create container error: %v", err)
		}

		got, err := runtime.Inspect(t.Context(), id)
		if err != nil {
			t.Fatalf("unexpected inspect error: %v", err)
		}

		want := container.RuntimeContainer{
			ID:    id,
			Image: image,
		}
		if got != want {
			t.Errorf("got %+v, want %+v", got, want)
		}
	})

	t.Run("duplicate container returns already exists", func(t *testing.T) {
		id := newContainerID(t)
		registerCleanup(t, id)

		if err := runtime.CreateContainer(t.Context(), id, image); err != nil {
			t.Fatalf("unexpected first create container error: %v", err)
		}

		err := runtime.CreateContainer(t.Context(), id, differentImage)
		if !errors.Is(err, container.ErrAlreadyExists) {
			t.Fatalf("got error %v, want %v", err, container.ErrAlreadyExists)
		}

		got, err := runtime.Inspect(t.Context(), id)
		if err != nil {
			t.Fatalf("unexpected inspect error: %v", err)
		}

		want := container.RuntimeContainer{
			ID:    id,
			Image: image,
		}
		if got != want {
			t.Errorf("got %+v, want unchanged %+v", got, want)
		}
	})

	t.Run("container can be deleted", func(t *testing.T) {
		id := newContainerID(t)
		registerCleanup(t, id)

		if err := runtime.CreateContainer(t.Context(), id, image); err != nil {
			t.Fatalf("unexpected create container error: %v", err)
		}
		if err := runtime.DeleteContainer(t.Context(), id); err != nil {
			t.Fatalf("unexpected delete container error: %v", err)
		}

		_, err := runtime.Inspect(t.Context(), id)
		if !errors.Is(err, container.ErrNotFound) {
			t.Fatalf("got error %v, want %v", err, container.ErrNotFound)
		}
	})

	t.Run("deleting missing container succeeds", func(t *testing.T) {
		id := newContainerID(t)
		registerCleanup(t, id)

		if err := runtime.DeleteContainer(t.Context(), id); err != nil {
			t.Fatalf("unexpected delete container error: %v", err)
		}
	})

	t.Run("container with task cannot be deleted", func(t *testing.T) {
		tests := []struct {
			name    string
			state   container.TaskState
			prepare func(*testing.T, string)
		}{
			{
				name:  "created",
				state: container.TaskStateCreated,
			},
			{
				name:  "running",
				state: container.TaskStateRunning,
				prepare: func(t *testing.T, id string) {
					t.Helper()
					if err := runtime.StartTask(t.Context(), id); err != nil {
						t.Fatalf("unexpected start task error: %v", err)
					}
				},
			},
			{
				name:  "stopped",
				state: container.TaskStateStopped,
				prepare: func(t *testing.T, id string) {
					t.Helper()
					if err := runtime.StartTask(t.Context(), id); err != nil {
						t.Fatalf("unexpected start task error: %v", err)
					}
					waitAfterStart()
					if err := runtime.StopTask(t.Context(), id); err != nil {
						t.Fatalf("unexpected stop task error: %v", err)
					}
				},
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				id := newContainerID(t)
				registerCleanup(t, id)

				if err := runtime.CreateContainer(t.Context(), id, image); err != nil {
					t.Fatalf("unexpected create container error: %v", err)
				}
				if _, err := runtime.CreateTask(t.Context(), id); err != nil {
					t.Fatalf("unexpected create task error: %v", err)
				}
				if tt.prepare != nil {
					tt.prepare(t, id)
				}

				err := runtime.DeleteContainer(t.Context(), id)
				if !errors.Is(err, container.ErrOperationNotAllowed) {
					t.Fatalf("got error %v, want %v", err, container.ErrOperationNotAllowed)
				}

				got, err := runtime.Inspect(t.Context(), id)
				if err != nil {
					t.Fatalf("unexpected inspect error: %v", err)
				}

				if got.ID != id {
					t.Errorf("got container ID %q, want unchanged %q", got.ID, id)
				}
				if got.Image != image {
					t.Errorf("got image %q, want unchanged %q", got.Image, image)
				}
				if got.Task == nil {
					t.Fatal("got nil task, want existing task")
				}
				if got.Task.State != tt.state {
					t.Errorf("got task state %q, want unchanged %q", got.Task.State, tt.state)
				}
			})
		}
	})

	t.Run("created task can be inspected", func(t *testing.T) {
		id := newContainerID(t)
		registerCleanup(t, id)

		if err := runtime.CreateContainer(t.Context(), id, image); err != nil {
			t.Fatalf("unexpected create container error: %v", err)
		}
		task, err := runtime.CreateTask(t.Context(), id)
		if err != nil {
			t.Fatalf("unexpected create task error: %v", err)
		}

		if task.PID == 0 {
			t.Error("got PID 0, want nonzero PID")
		}
		if task.NetNSPath == "" {
			t.Error("got empty network namespace path, want nonempty path")
		}
		if task.State != container.TaskStateCreated {
			t.Errorf(
				"got created task state %q, want %q",
				task.State,
				container.TaskStateCreated,
			)
		}

		got, err := runtime.Inspect(t.Context(), id)
		if err != nil {
			t.Fatalf("unexpected inspect error: %v", err)
		}

		want := container.RuntimeContainer{
			ID:    id,
			Image: image,
			Task: &container.RuntimeTask{
				PID:       task.PID,
				NetNSPath: task.NetNSPath,
				State:     container.TaskStateCreated,
			},
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %+v, want %+v", got, want)
		}
	})

	t.Run("creating task without container returns not found", func(t *testing.T) {
		id := newContainerID(t)
		registerCleanup(t, id)

		_, err := runtime.CreateTask(t.Context(), id)
		if !errors.Is(err, container.ErrNotFound) {
			t.Fatalf("got error %v, want %v", err, container.ErrNotFound)
		}
	})

	t.Run("duplicate task returns already exists", func(t *testing.T) {
		id := newContainerID(t)
		registerCleanup(t, id)

		if err := runtime.CreateContainer(t.Context(), id, image); err != nil {
			t.Fatalf("unexpected create container error: %v", err)
		}
		if _, err := runtime.CreateTask(t.Context(), id); err != nil {
			t.Fatalf("unexpected first create task error: %v", err)
		}

		_, err := runtime.CreateTask(t.Context(), id)
		if !errors.Is(err, container.ErrAlreadyExists) {
			t.Fatalf("got error %v, want %v", err, container.ErrAlreadyExists)
		}

		got, err := runtime.Inspect(t.Context(), id)
		if err != nil {
			t.Fatalf("unexpected inspect error: %v", err)
		}
		if got.Task == nil {
			t.Fatal("got nil task, want existing task")
		}
		if got.Task.State != container.TaskStateCreated {
			t.Errorf("got task state %q, want unchanged %q", got.Task.State, container.TaskStateCreated)
		}
	})

	t.Run("created task can be started", func(t *testing.T) {
		id := newContainerID(t)
		registerCleanup(t, id)

		if err := runtime.CreateContainer(t.Context(), id, image); err != nil {
			t.Fatalf("unexpected create container error: %v", err)
		}
		if _, err := runtime.CreateTask(t.Context(), id); err != nil {
			t.Fatalf("unexpected create task error: %v", err)
		}
		if err := runtime.StartTask(t.Context(), id); err != nil {
			t.Fatalf("unexpected start task error: %v", err)
		}

		got, err := runtime.Inspect(t.Context(), id)
		if err != nil {
			t.Fatalf("unexpected inspect error: %v", err)
		}
		if got.Task == nil {
			t.Fatal("got nil task, want running task")
		}
		if got.Task.State != container.TaskStateRunning {
			t.Errorf("got task state %q, want %q", got.Task.State, container.TaskStateRunning)
		}
	})

	t.Run("starting missing task returns not found", func(t *testing.T) {
		id := newContainerID(t)
		registerCleanup(t, id)

		if err := runtime.CreateContainer(t.Context(), id, image); err != nil {
			t.Fatalf("unexpected create container error: %v", err)
		}

		err := runtime.StartTask(t.Context(), id)
		if !errors.Is(err, container.ErrNotFound) {
			t.Fatalf("got error %v, want %v", err, container.ErrNotFound)
		}
	})

	t.Run("starting running task succeeds", func(t *testing.T) {
		id := newContainerID(t)
		registerCleanup(t, id)

		if err := runtime.CreateContainer(t.Context(), id, image); err != nil {
			t.Fatalf("unexpected create container error: %v", err)
		}
		if _, err := runtime.CreateTask(t.Context(), id); err != nil {
			t.Fatalf("unexpected create task error: %v", err)
		}
		if err := runtime.StartTask(t.Context(), id); err != nil {
			t.Fatalf("unexpected first start task error: %v", err)
		}

		if err := runtime.StartTask(t.Context(), id); err != nil {
			t.Fatalf("unexpected second start task error: %v", err)
		}

		got, err := runtime.Inspect(t.Context(), id)
		if err != nil {
			t.Fatalf("unexpected inspect error: %v", err)
		}
		if got.Task == nil {
			t.Fatal("got nil task, want running task")
		}
		if got.Task.State != container.TaskStateRunning {
			t.Errorf("got task state %q, want unchanged %q", got.Task.State, container.TaskStateRunning)
		}
	})

	t.Run("stopped task cannot be started", func(t *testing.T) {
		id := newContainerID(t)
		registerCleanup(t, id)

		if err := runtime.CreateContainer(t.Context(), id, image); err != nil {
			t.Fatalf("unexpected create container error: %v", err)
		}
		if _, err := runtime.CreateTask(t.Context(), id); err != nil {
			t.Fatalf("unexpected create task error: %v", err)
		}
		if err := runtime.StartTask(t.Context(), id); err != nil {
			t.Fatalf("unexpected start task error: %v", err)
		}
		waitAfterStart()
		if err := runtime.StopTask(t.Context(), id); err != nil {
			t.Fatalf("unexpected stop task error: %v", err)
		}

		err := runtime.StartTask(t.Context(), id)
		if !errors.Is(err, container.ErrOperationNotAllowed) {
			t.Fatalf("got error %v, want %v", err, container.ErrOperationNotAllowed)
		}

		got, err := runtime.Inspect(t.Context(), id)
		if err != nil {
			t.Fatalf("unexpected inspect error: %v", err)
		}
		if got.Task == nil {
			t.Fatal("got nil task, want stopped task")
		}
		if got.Task.State != container.TaskStateStopped {
			t.Errorf("got task state %q, want unchanged %q", got.Task.State, container.TaskStateStopped)
		}
	})

	t.Run("running task can be stopped", func(t *testing.T) {
		id := newContainerID(t)
		registerCleanup(t, id)

		if err := runtime.CreateContainer(t.Context(), id, image); err != nil {
			t.Fatalf("unexpected create container error: %v", err)
		}
		if _, err := runtime.CreateTask(t.Context(), id); err != nil {
			t.Fatalf("unexpected create task error: %v", err)
		}
		if err := runtime.StartTask(t.Context(), id); err != nil {
			t.Fatalf("unexpected start task error: %v", err)
		}
		waitAfterStart()
		if err := runtime.StopTask(t.Context(), id); err != nil {
			t.Fatalf("unexpected stop task error: %v", err)
		}

		got, err := runtime.Inspect(t.Context(), id)
		if err != nil {
			t.Fatalf("unexpected inspect error: %v", err)
		}
		if got.Task == nil {
			t.Fatal("got nil task, want stopped task")
		}
		if got.Task.State != container.TaskStateStopped {
			t.Errorf("got task state %q, want %q", got.Task.State, container.TaskStateStopped)
		}
	})

	t.Run("stopping task without container returns not found", func(t *testing.T) {
		id := newContainerID(t)
		registerCleanup(t, id)

		err := runtime.StopTask(t.Context(), id)
		if !errors.Is(err, container.ErrNotFound) {
			t.Fatalf("got error %v, want %v", err, container.ErrNotFound)
		}
	})

	t.Run("stopping missing task returns not found", func(t *testing.T) {
		id := newContainerID(t)
		registerCleanup(t, id)

		if err := runtime.CreateContainer(t.Context(), id, image); err != nil {
			t.Fatalf("unexpected create container error: %v", err)
		}

		err := runtime.StopTask(t.Context(), id)
		if !errors.Is(err, container.ErrNotFound) {
			t.Fatalf("got error %v, want %v", err, container.ErrNotFound)
		}
	})

	t.Run("created task cannot be stopped", func(t *testing.T) {
		id := newContainerID(t)
		registerCleanup(t, id)

		if err := runtime.CreateContainer(t.Context(), id, image); err != nil {
			t.Fatalf("unexpected create container error: %v", err)
		}
		if _, err := runtime.CreateTask(t.Context(), id); err != nil {
			t.Fatalf("unexpected create task error: %v", err)
		}

		err := runtime.StopTask(t.Context(), id)
		if !errors.Is(err, container.ErrOperationNotAllowed) {
			t.Fatalf("got error %v, want %v", err, container.ErrOperationNotAllowed)
		}

		got, err := runtime.Inspect(t.Context(), id)
		if err != nil {
			t.Fatalf("unexpected inspect error: %v", err)
		}
		if got.Task == nil {
			t.Fatal("got nil task, want created task")
		}
		if got.Task.State != container.TaskStateCreated {
			t.Errorf("got task state %q, want unchanged %q", got.Task.State, container.TaskStateCreated)
		}
	})

	t.Run("stopping stopped task succeeds", func(t *testing.T) {
		id := newContainerID(t)
		registerCleanup(t, id)

		if err := runtime.CreateContainer(t.Context(), id, image); err != nil {
			t.Fatalf("unexpected create container error: %v", err)
		}
		if _, err := runtime.CreateTask(t.Context(), id); err != nil {
			t.Fatalf("unexpected create task error: %v", err)
		}
		if err := runtime.StartTask(t.Context(), id); err != nil {
			t.Fatalf("unexpected start task error: %v", err)
		}
		waitAfterStart()
		if err := runtime.StopTask(t.Context(), id); err != nil {
			t.Fatalf("unexpected first stop task error: %v", err)
		}

		if err := runtime.StopTask(t.Context(), id); err != nil {
			t.Fatalf("unexpected second stop task error: %v", err)
		}

		got, err := runtime.Inspect(t.Context(), id)
		if err != nil {
			t.Fatalf("unexpected inspect error: %v", err)
		}
		if got.Task == nil {
			t.Fatal("got nil task, want stopped task")
		}
		if got.Task.State != container.TaskStateStopped {
			t.Errorf("got task state %q, want unchanged %q", got.Task.State, container.TaskStateStopped)
		}
	})

	t.Run("created and stopped tasks can be deleted", func(t *testing.T) {
		tests := []struct {
			name    string
			prepare func(*testing.T, string)
		}{
			{name: "created"},
			{
				name: "stopped",
				prepare: func(t *testing.T, id string) {
					t.Helper()
					if err := runtime.StartTask(t.Context(), id); err != nil {
						t.Fatalf("unexpected start task error: %v", err)
					}
					waitAfterStart()
					if err := runtime.StopTask(t.Context(), id); err != nil {
						t.Fatalf("unexpected stop task error: %v", err)
					}
				},
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				id := newContainerID(t)
				registerCleanup(t, id)

				if err := runtime.CreateContainer(t.Context(), id, image); err != nil {
					t.Fatalf("unexpected create container error: %v", err)
				}
				if _, err := runtime.CreateTask(t.Context(), id); err != nil {
					t.Fatalf("unexpected create task error: %v", err)
				}
				if tt.prepare != nil {
					tt.prepare(t, id)
				}

				if err := runtime.DeleteTask(t.Context(), id); err != nil {
					t.Fatalf("unexpected delete task error: %v", err)
				}

				got, err := runtime.Inspect(t.Context(), id)
				if err != nil {
					t.Fatalf("unexpected inspect error: %v", err)
				}

				want := container.RuntimeContainer{
					ID:    id,
					Image: image,
				}
				if got != want {
					t.Errorf("got %+v, want %+v", got, want)
				}
			})
		}
	})

	t.Run("running task cannot be deleted", func(t *testing.T) {
		id := newContainerID(t)
		registerCleanup(t, id)

		if err := runtime.CreateContainer(t.Context(), id, image); err != nil {
			t.Fatalf("unexpected create container error: %v", err)
		}
		if _, err := runtime.CreateTask(t.Context(), id); err != nil {
			t.Fatalf("unexpected create task error: %v", err)
		}
		if err := runtime.StartTask(t.Context(), id); err != nil {
			t.Fatalf("unexpected start task error: %v", err)
		}

		err := runtime.DeleteTask(t.Context(), id)
		if !errors.Is(err, container.ErrOperationNotAllowed) {
			t.Fatalf("got error %v, want %v", err, container.ErrOperationNotAllowed)
		}

		got, err := runtime.Inspect(t.Context(), id)
		if err != nil {
			t.Fatalf("unexpected inspect error: %v", err)
		}
		if got.Task == nil {
			t.Fatal("got nil task, want running task")
		}
		if got.Task.State != container.TaskStateRunning {
			t.Errorf("got task state %q, want unchanged %q", got.Task.State, container.TaskStateRunning)
		}
	})

	t.Run("deleting missing task succeeds", func(t *testing.T) {
		id := newContainerID(t)
		registerCleanup(t, id)

		if err := runtime.CreateContainer(t.Context(), id, image); err != nil {
			t.Fatalf("unexpected create container error: %v", err)
		}

		if err := runtime.DeleteTask(t.Context(), id); err != nil {
			t.Fatalf("unexpected delete task error: %v", err)
		}

		got, err := runtime.Inspect(t.Context(), id)
		if err != nil {
			t.Fatalf("unexpected inspect error: %v", err)
		}

		want := container.RuntimeContainer{
			ID:    id,
			Image: image,
		}
		if got != want {
			t.Errorf("got %+v, want unchanged %+v", got, want)
		}
	})

	t.Run("deleting task without container returns not found", func(t *testing.T) {
		id := newContainerID(t)
		registerCleanup(t, id)

		err := runtime.DeleteTask(t.Context(), id)
		if !errors.Is(err, container.ErrNotFound) {
			t.Fatalf("got error %v, want %v", err, container.ErrNotFound)
		}
	})
}
