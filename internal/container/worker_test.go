package container

import (
	"errors"
	"reflect"
	"testing"
	"testing/synctest"
	"time"
)

type failingRuntime struct {
	err error
}

func (r failingRuntime) CreateContainer(string, string) error {
	return r.err
}

func (r failingRuntime) Inspect(string) (RuntimeContainer, error) {
	panic("unexpected call to runtime.Inspect")
}

func (r failingRuntime) DeleteContainer(string) error {
	panic("unexpected call to runtime.Delete")
}

func (r failingRuntime) CreateTask(string) error {
	panic("unexpected call to runtime.CreateTask")
}

func (r failingRuntime) StartTask(string) error {
	panic("unexpected call to runtime.StartTask")
}

func (r failingRuntime) StopTask(string) error {
	panic("unexpected call to runtime.StopTask")
}

func (r failingRuntime) DeleteTask(string) error {
	panic("unexpected call to runtime.DeleteTask")
}

type runtimeDeleteFunc func(id string) error

func (runtimeDeleteFunc) CreateContainer(string, string) error {
	panic("unexpected call to runtime.Create")
}

func (runtimeDeleteFunc) Inspect(string) (RuntimeContainer, error) {
	panic("unexpected call to runtime.Inspect")
}

func (f runtimeDeleteFunc) DeleteContainer(id string) error {
	return f(id)
}

func (runtimeDeleteFunc) CreateTask(string) error {
	panic("unexpected call to runtime.CreateTask")
}

func (runtimeDeleteFunc) StartTask(string) error {
	panic("unexpected call to runtime.StartTask")
}

func (runtimeDeleteFunc) StopTask(string) error {
	panic("unexpected call to runtime.StopTask")
}

func (runtimeDeleteFunc) DeleteTask(string) error {
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
	if err := worker.handle(container.Name); err != nil {
		t.Fatalf("unexpected handle error: %v", err)
	}

	gotRuntime, err := runtime.Inspect(container.Name)
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
	if err := runtime.CreateContainer(container.Name, container.Image); err != nil {
		t.Fatalf("unexpected runtime create error: %v", err)
	}

	worker := newWorker(store, runtime, NewMemoryQueue())
	if err := worker.handle(container.Name); err != nil {
		t.Fatalf("unexpected handle error: %v", err)
	}

	gotRuntime, err := runtime.Inspect(container.Name)
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
	if err := runtime.CreateContainer(container.Name, container.Image); err != nil {
		t.Fatalf("unexpected runtime create error: %v", err)
	}
	if err := runtime.CreateTask(container.Name); err != nil {
		t.Fatalf("unexpected task create error: %v", err)
	}
	if err := runtime.StartTask(container.Name); err != nil {
		t.Fatalf("unexpected task start error: %v", err)
	}

	worker := newWorker(store, runtime, NewMemoryQueue())
	if err := worker.handle(container.Name); err != nil {
		t.Fatalf("unexpected handle error: %v", err)
	}

	gotRuntime, err := runtime.Inspect(container.Name)
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
	if err := runtime.CreateContainer(container.Name, container.Image); err != nil {
		t.Fatalf("unexpected runtime create error: %v", err)
	}

	worker := newWorker(store, runtime, NewMemoryQueue())
	if err := worker.handle(container.Name); err != nil {
		t.Fatalf("unexpected handle error: %v", err)
	}

	_, err := runtime.Inspect(container.Name)
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
	if err := worker.handle(container.Name); err != nil {
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
	if _, err := worker.handleNext(); err != nil {
		t.Fatalf("unexpected handle next error: %v", err)
	}

	gotRuntime, err := runtime.Inspect(container.Name)
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

func TestWorkerHandleNextReportsRuntimeFailure(t *testing.T) {
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
	if _, err := worker.handleNext(); !errors.Is(err, wantErr) {
		t.Fatalf("got error %v, want %v", err, wantErr)
	}

	got, err := store.Get(container.Name)
	if err != nil {
		t.Fatalf("unexpected store get error: %v", err)
	}
	if got != container {
		t.Errorf("got stored container %+v, want unchanged %+v", got, container)
	}
}

func TestWorkerHandleNextShutdown(t *testing.T) {
	queue := NewMemoryQueue()
	queue.Shutdown()

	worker := newWorker(NewMemoryStore(), NewMemoryRuntime(), queue)

	shutdown, err := worker.handleNext()
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
			errCh <- worker.run()
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
				if err := runtime.CreateContainer(tt.container.Name, tt.container.Image); err != nil {
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
			_, err := runtime.Inspect(tt.container.Name)
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

func TestWorkerHandleNextRequeuesFailedWorkAfterDelay(t *testing.T) {
	const wantRetryDelay = time.Second

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

	shutdown, err := worker.handleNext()
	if shutdown {
		t.Fatal("got shutdown true, want false")
	}
	if !errors.Is(err, wantErr) {
		t.Fatalf("got error %v, want %v", err, wantErr)
	}

	// this is a "mechanical" test. we want the worker to just call queue AddAfter if there is an error.
	// And that Done is called after processing.
	if queue.addAfterCalls != 1 {
		t.Fatalf("add after calls: %d, want 1", queue.addAfterCalls)
	}

	if queue.addAfterName != "hello" {
		t.Fatalf("added name %q, want %q", queue.addAfterName, "hello")
	}

	if queue.addAfterDelay != wantRetryDelay {
		t.Fatalf("add after delay: %v, want %v", queue.addAfterDelay, wantRetryDelay)
	}

	if queue.doneCalls != 1 {
		t.Fatalf("done calls: %d, want 1", queue.doneCalls)
	}

	if queue.doneName != "hello" {
		t.Fatalf("done name %q, want %q", queue.doneName, "hello")
	}
}
