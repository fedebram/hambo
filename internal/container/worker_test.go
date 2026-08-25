package container

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"testing/synctest"
	"time"
)

type failingRuntime struct {
	err error
}

func (r failingRuntime) CreateContainer(context.Context, string, string) error {
	return r.err
}

func (r failingRuntime) Inspect(context.Context, string) (RuntimeContainer, error) {
	panic("unexpected call to runtime.Inspect")
}

func (r failingRuntime) DeleteContainer(context.Context, string) error {
	panic("unexpected call to runtime.Delete")
}

func (r failingRuntime) CreateTask(context.Context, string) error {
	panic("unexpected call to runtime.CreateTask")
}

func (r failingRuntime) StartTask(context.Context, string) error {
	panic("unexpected call to runtime.StartTask")
}

func (r failingRuntime) StopTask(context.Context, string) error {
	panic("unexpected call to runtime.StopTask")
}

func (r failingRuntime) DeleteTask(context.Context, string) error {
	panic("unexpected call to runtime.DeleteTask")
}

type runtimeDeleteFunc func(id string) error

func (runtimeDeleteFunc) CreateContainer(context.Context, string, string) error {
	panic("unexpected call to runtime.Create")
}

func (runtimeDeleteFunc) Inspect(context.Context, string) (RuntimeContainer, error) {
	panic("unexpected call to runtime.Inspect")
}

func (f runtimeDeleteFunc) DeleteContainer(_ context.Context, id string) error {
	return f(id)
}

func (runtimeDeleteFunc) CreateTask(context.Context, string) error {
	panic("unexpected call to runtime.CreateTask")
}

func (runtimeDeleteFunc) StartTask(context.Context, string) error {
	panic("unexpected call to runtime.StartTask")
}

func (runtimeDeleteFunc) StopTask(context.Context, string) error {
	panic("unexpected call to runtime.StopTask")
}

func (runtimeDeleteFunc) DeleteTask(context.Context, string) error {
	panic("unexpected call to runtime.DeleteTask")
}

func TestWorkerHandlesCreatingContainer(t *testing.T) {
	store := NewMemoryStore()
	runtime := NewMemoryRuntime()

	container := Container{
		Name:  "hello",
		Image: "docker.io/library/alpine:latest",
		State: StateCreating,
	}
	if err := store.Create(container); err != nil {
		t.Fatalf("unexpected store create error: %v", err)
	}

	worker := newWorker(store, runtime, NewMemoryQueue())
	if err := worker.handle(t.Context(), container.Name); err != nil {
		t.Fatalf("unexpected handle error: %v", err)
	}

	gotRuntime, err := runtime.Inspect(t.Context(), container.Name)
	if err != nil {
		t.Fatalf("unexpected runtime inspect error: %v", err)
	}
	wantRuntime := RuntimeContainer{
		ID:    container.Name,
		Image: container.Image,
	}
	if gotRuntime != wantRuntime {
		t.Errorf("got runtime container %+v, want %+v", gotRuntime, wantRuntime)
	}

	got, err := store.Get(container.Name)
	if err != nil {
		t.Fatalf("unexpected store get error: %v", err)
	}

	want := container
	want.State = StateCreated
	if got != want {
		t.Errorf("got stored container %+v, want %+v", got, want)
	}
}

func TestWorkerHandlesStartingContainer(t *testing.T) {
	store := NewMemoryStore()
	runtime := NewMemoryRuntime()

	container := Container{
		Name:  "hello",
		Image: "docker.io/library/alpine:latest",
		State: StateStarting,
	}
	if err := store.Create(container); err != nil {
		t.Fatalf("unexpected store create error: %v", err)
	}
	if err := runtime.CreateContainer(t.Context(), container.Name, container.Image); err != nil {
		t.Fatalf("unexpected runtime create error: %v", err)
	}

	worker := newWorker(store, runtime, NewMemoryQueue())
	if err := worker.handle(t.Context(), container.Name); err != nil {
		t.Fatalf("unexpected handle error: %v", err)
	}

	gotRuntime, err := runtime.Inspect(t.Context(), container.Name)
	if err != nil {
		t.Fatalf("unexpected runtime inspect error: %v", err)
	}
	wantRuntime := RuntimeContainer{
		ID:    container.Name,
		Image: container.Image,
		Task: &RuntimeTask{
			State: TaskStateRunning,
		},
	}
	if !reflect.DeepEqual(gotRuntime, wantRuntime) {
		// task is a pointer... it prints the pointer address...
		// TODO: a better way to print and compare. google/go-cmp package?
		t.Errorf("got runtime container %+v, want %+v", gotRuntime, wantRuntime)
	}

	got, err := store.Get(container.Name)
	if err != nil {
		t.Fatalf("unexpected store get error: %v", err)
	}
	want := container
	want.State = StateRunning
	if got != want {
		t.Errorf("got stored container %+v, want %+v", got, want)
	}
}

func TestWorkerHandlesStoppingContainer(t *testing.T) {
	store := NewMemoryStore()
	runtime := NewMemoryRuntime()

	container := Container{
		Name:  "hello",
		Image: "docker.io/library/alpine:latest",
		State: StateStopping,
	}
	if err := store.Create(container); err != nil {
		t.Fatalf("unexpected store create error: %v", err)
	}
	if err := runtime.CreateContainer(t.Context(), container.Name, container.Image); err != nil {
		t.Fatalf("unexpected runtime create error: %v", err)
	}
	if err := runtime.CreateTask(t.Context(), container.Name); err != nil {
		t.Fatalf("unexpected task create error: %v", err)
	}
	if err := runtime.StartTask(t.Context(), container.Name); err != nil {
		t.Fatalf("unexpected task start error: %v", err)
	}

	worker := newWorker(store, runtime, NewMemoryQueue())
	if err := worker.handle(t.Context(), container.Name); err != nil {
		t.Fatalf("unexpected handle error: %v", err)
	}

	gotRuntime, err := runtime.Inspect(t.Context(), container.Name)
	if err != nil {
		t.Fatalf("unexpected runtime inspect error: %v", err)
	}
	wantRuntime := RuntimeContainer{
		ID:    container.Name,
		Image: container.Image,
	}
	if !reflect.DeepEqual(gotRuntime, wantRuntime) {
		t.Errorf("got runtime container %+v, want %+v", gotRuntime, wantRuntime)
	}

	got, err := store.Get(container.Name)
	if err != nil {
		t.Fatalf("unexpected store get error: %v", err)
	}
	want := container
	want.State = StateStopped
	if got != want {
		t.Errorf("got stored container %+v, want %+v", got, want)
	}
}

func TestWorkerHandlesDeletingContainer(t *testing.T) {
	store := NewMemoryStore()
	runtime := NewMemoryRuntime()

	container := Container{
		Name:  "hello",
		Image: "docker.io/library/alpine:latest",
		State: StateCreated,
		DeletionTimestamp: time.Date(
			2026, time.July, 19, 15, 0, 0, 0, time.UTC,
		),
	}
	if err := store.Create(container); err != nil {
		t.Fatalf("unexpected store create error: %v", err)
	}
	if err := runtime.CreateContainer(t.Context(), container.Name, container.Image); err != nil {
		t.Fatalf("unexpected runtime create error: %v", err)
	}

	worker := newWorker(store, runtime, NewMemoryQueue())
	if err := worker.handle(t.Context(), container.Name); err != nil {
		t.Fatalf("unexpected handle error: %v", err)
	}

	_, err := runtime.Inspect(t.Context(), container.Name)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("got runtime inspect error %v, want %v", err, ErrNotFound)
	}

	_, err = store.Get(container.Name)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("got store error %v, want %v", err, ErrNotFound)
	}
}

func TestWorkerMarksContainerDeletingBeforeRuntimeDeletion(t *testing.T) {
	store := NewMemoryStore()
	container := Container{
		Name:              "hello",
		State:             StateCreated,
		DeletionTimestamp: time.Date(2026, time.July, 19, 15, 0, 0, 0, time.UTC),
	}
	if err := store.Create(container); err != nil {
		t.Fatalf("unexpected store create error: %v", err)
	}

	deleteCalled := false
	runtime := runtimeDeleteFunc(func(name string) error {
		deleteCalled = true

		got, err := store.Get(name)
		if err != nil {
			t.Errorf("get container during runtime deletion: %v", err)
			return nil
		}
		if got.State != StateDeleting {
			t.Errorf("got state %q during runtime deletion, want %q", got.State, StateDeleting)
		}
		return nil
	})

	worker := newWorker(store, runtime, NewMemoryQueue())
	if err := worker.handle(t.Context(), container.Name); err != nil {
		t.Fatalf("unexpected handle error: %v", err)
	}
	if !deleteCalled {
		t.Fatal("runtime delete was not called")
	}
}

func TestWorkerHandlesNextQueuedContainer(t *testing.T) {
	store := NewMemoryStore()
	runtime := NewMemoryRuntime()

	container := Container{
		Name:  "hello",
		Image: "docker.io/library/alpine:latest",
		State: StateCreating,
	}
	if err := store.Create(container); err != nil {
		t.Fatalf("unexpected store create error: %v", err)
	}

	queue := &recordingQueue{next: container.Name}

	worker := newWorker(store, runtime, queue)
	if _, err := worker.handleNext(t.Context()); err != nil {
		t.Fatalf("unexpected handle next error: %v", err)
	}

	gotRuntime, err := runtime.Inspect(t.Context(), container.Name)
	if err != nil {
		t.Fatalf("unexpected runtime inspect error: %v", err)
	}
	wantRuntime := RuntimeContainer{
		ID:    container.Name,
		Image: container.Image,
	}
	if gotRuntime != wantRuntime {
		t.Errorf("got runtime container %+v, want %+v", gotRuntime, wantRuntime)
	}

	got, err := store.Get(container.Name)
	if err != nil {
		t.Fatalf("unexpected store get error: %v", err)
	}

	want := container
	want.State = StateCreated
	if got != want {
		t.Errorf("got stored container %+v, want %+v", got, want)
	}

	if queue.doneCalls != 1 {
		t.Fatalf("done calls: %d, want 1", queue.doneCalls)
	}
	if queue.doneName != container.Name {
		t.Fatalf("done name %q, want %q", queue.doneName, container.Name)
	}
}

func TestWorkerHandleNextRecordsAndReportsRuntimeFailure(t *testing.T) {
	wantErr := errors.New("runtime unavailable")
	store := NewMemoryStore()
	queue := NewMemoryQueue()
	defer queue.Shutdown()

	container := Container{
		Name:  "hello",
		Image: "docker.io/library/alpine:latest",
		State: StateCreating,
	}
	if err := store.Create(container); err != nil {
		t.Fatalf("unexpected store create error: %v", err)
	}
	queue.Add(container.Name)

	worker := newWorker(store, failingRuntime{err: wantErr}, queue)
	if _, err := worker.handleNext(t.Context()); !errors.Is(err, wantErr) {
		t.Fatalf("got error %v, want %v", err, wantErr)
	}

	want := container
	want.Error = wantErr.Error()

	got, err := store.Get(container.Name)
	if err != nil {
		t.Fatalf("unexpected store get error: %v", err)
	}
	if got != want {
		t.Errorf("got stored container %+v, want %+v", got, want)
	}
}

func TestWorkerHandleNextShutdown(t *testing.T) {
	queue := NewMemoryQueue()
	queue.Shutdown()

	worker := newWorker(NewMemoryStore(), NewMemoryRuntime(), queue)

	shutdown, err := worker.handleNext(t.Context())
	if err != nil {
		t.Fatalf("unexpected handle next error: %v", err)
	}
	if !shutdown {
		t.Fatal("got shutdown false, want true")
	}
}

func TestWorkerRunHandlesQueuedContainers(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		store := NewMemoryStore()
		runtime := NewMemoryRuntime()
		queue := NewMemoryQueue()

		worker := newWorker(store, runtime, queue)

		errCh := make(chan error, 1)

		go func() {
			errCh <- worker.run(t.Context())
		}()

		deletionTime := time.Date(2026, time.July, 19, 15, 0, 0, 0, time.UTC)
		tests := []struct {
			container       Container
			runtimeExists   bool
			wantRuntime     bool
			wantStoredState State
		}{
			{
				container: Container{
					Name:  "hello",
					Image: "docker.io/library/alpine:latest",
					State: StateCreating,
				},
				wantRuntime:     true,
				wantStoredState: StateCreated,
			},
			{
				container: Container{
					Name:              "database",
					Image:             "docker.io/library/postgres:latest",
					State:             StateCreated,
					DeletionTimestamp: deletionTime,
				},
				runtimeExists: true,
			},
		}

		for _, tt := range tests {
			if err := store.Create(tt.container); err != nil {
				t.Fatalf("unexpected store create error: %v", err)
			}
			if tt.runtimeExists {
				if err := runtime.CreateContainer(t.Context(), tt.container.Name, tt.container.Image); err != nil {
					t.Fatalf("unexpected runtime create error: %v", err)
				}
			}
			queue.Add(tt.container.Name)
		}

		// wait on the worker to process all the containers
		synctest.Wait()

		if got := queue.len(); got != 0 {
			t.Fatalf("got queue length: %d, want 0", got)
		}

		// the queue is now empty, the worker must block and not exit.
		select {
		case err := <-errCh:
			t.Fatalf("worker returned before shutdown: %v", err)
		default:
		}

		for _, tt := range tests {
			_, err := runtime.Inspect(t.Context(), tt.container.Name)
			if err != nil && !errors.Is(err, ErrNotFound) {
				t.Fatalf("unexpected runtime inspect error: %v", err)
			}
			runtimeExists := err == nil
			if runtimeExists != tt.wantRuntime {
				t.Errorf(
					"container %q runtime existence: got %t, want %t",
					tt.container.Name,
					runtimeExists,
					tt.wantRuntime,
				)
			}

			got, err := store.Get(tt.container.Name)
			if tt.wantStoredState == "" {
				if !errors.Is(err, ErrNotFound) {
					t.Errorf(
						"container %q store error: got %v, want %v",
						tt.container.Name,
						err,
						ErrNotFound,
					)
				}
				continue
			}
			if err != nil {
				t.Fatalf("unexpected store get error: %v", err)
			}

			want := tt.container
			want.State = tt.wantStoredState
			if got != want {
				t.Errorf("got stored container %+v, want %+v", got, want)
			}
		}

		queue.Shutdown()

		synctest.Wait()

		select {
		case err := <-errCh:
			if err != nil {
				t.Fatalf("unexpected worker run error: %v", err)
			}
		default:
			t.Fatal("worker did not return after shutdown")
		}
	})
}

func TestWorkerHandleNextDoesNotRequeueFailedWork(t *testing.T) {
	wantErr := errors.New("runtime unavailable")
	container := Container{Name: "hello", State: StateCreating}
	store := NewMemoryStore()
	if err := store.Create(container); err != nil {
		t.Fatalf("unexpected store create error: %v", err)
	}
	queue := &recordingQueue{next: container.Name}

	worker := newWorker(
		store,
		failingRuntime{err: wantErr},
		queue,
	)

	shutdown, err := worker.handleNext(t.Context())
	if shutdown {
		t.Fatal("got shutdown true, want false")
	}
	if !errors.Is(err, wantErr) {
		t.Fatalf("got error %v, want %v", err, wantErr)
	}

	if queue.addAfterCalls != 0 {
		t.Fatalf("add after calls: %d, want 0", queue.addAfterCalls)
	}

	if queue.doneCalls != 1 {
		t.Fatalf("done calls: %d, want 1", queue.doneCalls)
	}

	if queue.doneName != "hello" {
		t.Fatalf("done name %q, want %q", queue.doneName, "hello")
	}
}
