package container

import (
	"errors"
	"reflect"
	"testing"
)

func TestInspectMissingContainer(t *testing.T) {
	runtime := NewMemoryRuntime()

	_, err := runtime.Inspect("hello")

	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("got %v error, want %v error", err, ErrNotFound)
	}
}

func TestCreateAndInspectContainer(t *testing.T) {
	runtime := NewMemoryRuntime()

	want := RuntimeContainer{
		ID:    "hello",
		Image: "docker.io/library/alpine:latest",
	}

	if err := runtime.CreateContainer(want.ID, want.Image); err != nil {
		t.Fatalf("unexpected runtime create error: %v", err)
	}

	got, err := runtime.Inspect(want.ID)
	if err != nil {
		t.Fatalf("unexpected runtime inspect error: %v", err)
	}

	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestCreateAndInspectTask(t *testing.T) {
	runtime := NewMemoryRuntime()
	if err := runtime.CreateContainer("hello", "alpine"); err != nil {
		t.Fatalf("unexpected container create error: %v", err)
	}

	if err := runtime.CreateTask("hello"); err != nil {
		t.Fatalf("unexpected task create error: %v", err)
	}

	got, err := runtime.Inspect("hello")
	if err != nil {
		t.Fatalf("unexpected inspect error: %v", err)
	}
	if got.Task == nil {
		t.Fatal("got nil task, want created task")
	}
	if got.Task.State != TaskStateCreated {
		t.Errorf("got task state %q, want %q", got.Task.State, TaskStateCreated)
	}
}
func TestCreateContainerRejectsDuplicate(t *testing.T) {
	runtime := NewMemoryRuntime()
	if err := runtime.CreateContainer("hello", "alpine"); err != nil {
		t.Fatalf("unexpected first create error: %v", err)
	}

	err := runtime.CreateContainer("hello", "redis")

	if !errors.Is(err, ErrAlreadyExists) {
		t.Errorf("got error %v, want %v", err, ErrAlreadyExists)
	}
}

func TestDeleteContainer(t *testing.T) {
	runtime := NewMemoryRuntime()
	if err := runtime.CreateContainer("hello", "alpine"); err != nil {
		t.Fatalf("unexpected create error: %v", err)
	}
	if err := runtime.DeleteContainer("hello"); err != nil {
		t.Fatalf("unexpected delete error: %v", err)
	}

	_, err := runtime.Inspect("hello")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("got %v error, want %v error", err, ErrNotFound)
	}
}

func TestDeleteMissingContainerIsNoOp(t *testing.T) {
	runtime := NewMemoryRuntime()
	// basically runtime delete is idempotent
	if err := runtime.DeleteContainer("hello"); err != nil {
		t.Fatalf("unexpected delete container error: %v", err)
	}
}

func TestDeleteContainerRejectsExistingTask(t *testing.T) {
	tests := []struct {
		name    string
		prepare func(t *testing.T, runtime *MemoryRuntime)
		state   TaskState
	}{
		{
			name:  "created",
			state: TaskStateCreated,
		},
		{
			name: "running",
			prepare: func(t *testing.T, runtime *MemoryRuntime) {
				t.Helper()
				if err := runtime.StartTask("hello"); err != nil {
					t.Fatalf("unexpected task start error: %v", err)
				}
			},
			state: TaskStateRunning,
		},
		{
			name: "stopped",
			prepare: func(t *testing.T, runtime *MemoryRuntime) {
				t.Helper()
				if err := runtime.StartTask("hello"); err != nil {
					t.Fatalf("unexpected task start error: %v", err)
				}
				if err := runtime.StopTask("hello"); err != nil {
					t.Fatalf("unexpected task stop error: %v", err)
				}
			},
			state: TaskStateStopped,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runtime := NewMemoryRuntime()
			if err := runtime.CreateContainer("hello", "alpine"); err != nil {
				t.Fatalf("unexpected container create error: %v", err)
			}
			if err := runtime.CreateTask("hello"); err != nil {
				t.Fatalf("unexpected task create error: %v", err)
			}
			if tt.prepare != nil {
				tt.prepare(t, runtime)
			}

			err := runtime.DeleteContainer("hello")
			if !errors.Is(err, ErrOperationNotAllowed) {
				t.Fatalf("got error %v, want %v", err, ErrOperationNotAllowed)
			}

			want := RuntimeContainer{
				ID:    "hello",
				Image: "alpine",
				Task: &RuntimeTask{
					State: tt.state,
				},
			}

			got, err := runtime.Inspect("hello")
			if err != nil {
				t.Fatalf("unexpected inspect error: %v", err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("got %+v, want unchanged %+v", got, want)
			}
		})
	}
}

func TestCreateTaskWithoutContainerReturnsNotFound(t *testing.T) {
	runtime := NewMemoryRuntime()

	err := runtime.CreateTask("hello")

	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("got error %v, want %v", err, ErrNotFound)
	}
}

func TestCreateTaskRejectsDuplicate(t *testing.T) {
	runtime := NewMemoryRuntime()

	if err := runtime.CreateContainer("hello", "alpine"); err != nil {
		t.Fatalf("unexpected container create error: %v", err)
	}

	if err := runtime.CreateTask("hello"); err != nil {
		t.Fatalf("unexpected task create error: %v", err)
	}

	if err := runtime.CreateTask("hello"); !errors.Is(err, ErrAlreadyExists) {
		t.Fatalf("got %v error, want %v error", err, ErrAlreadyExists)
	}
}

func TestStartTask(t *testing.T) {
	runtime := NewMemoryRuntime()
	if err := runtime.CreateContainer("hello", "alpine"); err != nil {
		t.Fatalf("unexpected container create error: %v", err)
	}
	if err := runtime.CreateTask("hello"); err != nil {
		t.Fatalf("unexpected task create error: %v", err)
	}

	if err := runtime.StartTask("hello"); err != nil {
		t.Fatalf("unexpected task start error: %v", err)
	}

	got, err := runtime.Inspect("hello")
	if err != nil {
		t.Fatalf("unexpected inspect error: %v", err)
	}
	if got.Task == nil {
		t.Fatal("got nil task, want running task")
	}
	if got.Task.State != TaskStateRunning {
		t.Errorf("got task state %q, want %q", got.Task.State, TaskStateRunning)
	}
}

func TestStartMissingTaskReturnsNotFound(t *testing.T) {
	runtime := NewMemoryRuntime()
	if err := runtime.CreateContainer("hello", "alpine"); err != nil {
		t.Fatalf("unexpected container create error: %v", err)
	}

	err := runtime.StartTask("hello")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("got error %v, want %v", err, ErrNotFound)
	}
}

func TestStartRunningTaskIsNoOp(t *testing.T) {
	runtime := NewMemoryRuntime()
	if err := runtime.CreateContainer("hello", "alpine"); err != nil {
		t.Fatalf("unexpected container create error: %v", err)
	}
	if err := runtime.CreateTask("hello"); err != nil {
		t.Fatalf("unexpected task create error: %v", err)
	}
	if err := runtime.StartTask("hello"); err != nil {
		t.Fatalf("unexpected first task start error: %v", err)
	}

	if err := runtime.StartTask("hello"); err != nil {
		t.Fatalf("unexpected second task start error: %v", err)
	}

	got, err := runtime.Inspect("hello")
	if err != nil {
		t.Fatalf("unexpected inspect error: %v", err)
	}
	if got.Task == nil {
		t.Fatal("got nil task, want running task")
	}
	if got.Task.State != TaskStateRunning {
		t.Errorf("got task state %q, want unchanged %q", got.Task.State, TaskStateRunning)
	}
}

func TestStartStoppedTaskReturnsOperationNotAllowed(t *testing.T) {
	runtime := NewMemoryRuntime()
	if err := runtime.CreateContainer("hello", "alpine"); err != nil {
		t.Fatalf("unexpected container create error: %v", err)
	}
	if err := runtime.CreateTask("hello"); err != nil {
		t.Fatalf("unexpected task create error: %v", err)
	}
	if err := runtime.StartTask("hello"); err != nil {
		t.Fatalf("unexpected task start error: %v", err)
	}
	if err := runtime.StopTask("hello"); err != nil {
		t.Fatalf("unexpected task stop error: %v", err)
	}

	err := runtime.StartTask("hello")
	if !errors.Is(err, ErrOperationNotAllowed) {
		t.Fatalf("got error %v, want %v", err, ErrOperationNotAllowed)
	}

	got, err := runtime.Inspect("hello")
	if err != nil {
		t.Fatalf("unexpected inspect error: %v", err)
	}
	if got.Task == nil {
		t.Fatal("got nil task, want stopped task")
	}
	if got.Task.State != TaskStateStopped {
		t.Errorf("got task state %q, want unchanged %q", got.Task.State, TaskStateStopped)
	}
}

func TestStopTask(t *testing.T) {
	runtime := NewMemoryRuntime()
	if err := runtime.CreateContainer("hello", "alpine"); err != nil {
		t.Fatalf("unexpected container create error: %v", err)
	}
	if err := runtime.CreateTask("hello"); err != nil {
		t.Fatalf("unexpected task create error: %v", err)
	}
	if err := runtime.StartTask("hello"); err != nil {
		t.Fatalf("unexpected task start error: %v", err)
	}

	if err := runtime.StopTask("hello"); err != nil {
		t.Fatalf("unexpected task stop error: %v", err)
	}

	got, err := runtime.Inspect("hello")
	if err != nil {
		t.Fatalf("unexpected inspect error: %v", err)
	}
	if got.Task == nil {
		t.Fatal("got nil task, want stopped task")
	}
	if got.Task.State != TaskStateStopped {
		t.Errorf("got task state %q, want %q", got.Task.State, TaskStateStopped)
	}
}

func TestStopMissingContainerAndTaskReturnsNotFound(t *testing.T) {
	runtime := NewMemoryRuntime()

	if err := runtime.StopTask("hello"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("got error %v, want %v", err, ErrNotFound)
	}

	if err := runtime.CreateContainer("hello", "alpine"); err != nil {
		t.Fatalf("unexpected container create error: %v", err)
	}

	if err := runtime.StopTask("hello"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("got error %v, want %v", err, ErrNotFound)
	}
}

func TestStopCreatedTaskReturnsOperationNotAllowed(t *testing.T) {
	runtime := NewMemoryRuntime()
	if err := runtime.CreateContainer("hello", "alpine"); err != nil {
		t.Fatalf("unexpected container create error: %v", err)
	}
	if err := runtime.CreateTask("hello"); err != nil {
		t.Fatalf("unexpected task create error: %v", err)
	}

	err := runtime.StopTask("hello")
	if !errors.Is(err, ErrOperationNotAllowed) {
		t.Fatalf("got error %v, want %v", err, ErrOperationNotAllowed)
	}

	got, err := runtime.Inspect("hello")
	if err != nil {
		t.Fatalf("unexpected inspect error: %v", err)
	}
	if got.Task == nil {
		t.Fatal("got nil task, want created task")
	}
	if got.Task.State != TaskStateCreated {
		t.Errorf("got task state %q, want unchanged %q", got.Task.State, TaskStateCreated)
	}
}

func TestStopStoppedTaskIsNoOp(t *testing.T) {
	runtime := NewMemoryRuntime()
	if err := runtime.CreateContainer("hello", "alpine"); err != nil {
		t.Fatalf("unexpected container create error: %v", err)
	}
	if err := runtime.CreateTask("hello"); err != nil {
		t.Fatalf("unexpected task create error: %v", err)
	}
	if err := runtime.StartTask("hello"); err != nil {
		t.Fatalf("unexpected task start error: %v", err)
	}
	if err := runtime.StopTask("hello"); err != nil {
		t.Fatalf("unexpected first task stop error: %v", err)
	}

	if err := runtime.StopTask("hello"); err != nil {
		t.Fatalf("unexpected second task stop error: %v", err)
	}

	got, err := runtime.Inspect("hello")
	if err != nil {
		t.Fatalf("unexpected inspect error: %v", err)
	}
	if got.Task == nil {
		t.Fatal("got nil task, want stopped task")
	}
	if got.Task.State != TaskStateStopped {
		t.Errorf("got task state %q, want unchanged %q", got.Task.State, TaskStateStopped)
	}
}

func TestDeleteTask(t *testing.T) {
	tests := []struct {
		name    string
		prepare func(t *testing.T, runtime *MemoryRuntime)
	}{
		{name: "created"},
		{
			name: "stopped",
			prepare: func(t *testing.T, runtime *MemoryRuntime) {
				t.Helper()
				if err := runtime.StartTask("hello"); err != nil {
					t.Fatalf("unexpected task start error: %v", err)
				}
				if err := runtime.StopTask("hello"); err != nil {
					t.Fatalf("unexpected task stop error: %v", err)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runtime := NewMemoryRuntime()
			if err := runtime.CreateContainer("hello", "alpine"); err != nil {
				t.Fatalf("unexpected container create error: %v", err)
			}
			if err := runtime.CreateTask("hello"); err != nil {
				t.Fatalf("unexpected task create error: %v", err)
			}
			if tt.prepare != nil {
				tt.prepare(t, runtime)
			}

			if err := runtime.DeleteTask("hello"); err != nil {
				t.Fatalf("unexpected task delete error: %v", err)
			}

			got, err := runtime.Inspect("hello")
			if err != nil {
				t.Fatalf("unexpected inspect error: %v", err)
			}
			if got.Task != nil {
				t.Errorf("got task %+v, want nil", got.Task)
			}
		})
	}
}

func TestDeleteTaskRejectsRunningTask(t *testing.T) {
	runtime := NewMemoryRuntime()
	if err := runtime.CreateContainer("hello", "alpine"); err != nil {
		t.Fatalf("unexpected container create error: %v", err)
	}
	if err := runtime.CreateTask("hello"); err != nil {
		t.Fatalf("unexpected task create error: %v", err)
	}
	if err := runtime.StartTask("hello"); err != nil {
		t.Fatalf("unexpected task start error: %v", err)
	}

	err := runtime.DeleteTask("hello")
	if !errors.Is(err, ErrOperationNotAllowed) {
		t.Fatalf("got error %v, want %v", err, ErrOperationNotAllowed)
	}

	got, err := runtime.Inspect("hello")
	if err != nil {
		t.Fatalf("unexpected inspect error: %v", err)
	}
	if got.Task == nil {
		t.Fatal("got nil task, want running task unchanged")
	}
	if got.Task.State != TaskStateRunning {
		t.Errorf("got task state %q, want unchanged %q", got.Task.State, TaskStateRunning)
	}
}

func TestDeleteMissingTaskIsNoOp(t *testing.T) {
	runtime := NewMemoryRuntime()
	if err := runtime.CreateContainer("hello", "alpine"); err != nil {
		t.Fatalf("unexpected delete task error: %v", err)
	}
	if err := runtime.DeleteTask("hello"); err != nil {
		t.Fatalf("unexpected delete task error: %v", err)
	}
}

func TestDeleteTaskOnMissingContainerReturnsNotFound(t *testing.T) {
	runtime := NewMemoryRuntime()
	if err := runtime.DeleteTask("hello"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("got %v error, want %v error", err, ErrNotFound)
	}
}
