package container

import (
	"errors"
	"testing"
	"testing/synctest"
	"time"
)

type failingRuntime struct {
	err error
}

func (r failingRuntime) Create(string) error {
	return r.err
}

func (r failingRuntime) Inspect(string) (bool, error) {
	panic("unexpected call to runtime.Inspect")
}

func (r failingRuntime) Delete(string) error {
	panic("unexpected call to runtime.Delete")
}

type runtimeDeleteFunc func(name string) error

func (runtimeDeleteFunc) Create(string) error {
	panic("unexpected call to runtime.Create")
}

func (runtimeDeleteFunc) Inspect(string) (bool, error) {
	panic("unexpected call to runtime.Inspect")
}

func (f runtimeDeleteFunc) Delete(name string) error {
	return f(name)
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

	found, err := runtime.Inspect(container.Name)
	if err != nil {
		t.Fatalf("unexpected runtime inspect error: %v", err)
	}
	if !found {
		t.Error("container missing from runtime after handling")
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

func TestWorkerHandlesContainerAlreadyInRuntime(t *testing.T) {
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

	if err := runtime.Create(container.Name); err != nil {
		t.Fatalf("unexpected runtime create error: %v", err)
	}

	worker := newWorker(store, runtime, NewMemoryQueue())
	if err := worker.handle(container.Name); err != nil {
		t.Fatalf("unexpected handle error: %v", err)
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
	if err := runtime.Create(container.Name); err != nil {
		t.Fatalf("unexpected runtime create error: %v", err)
	}

	worker := newWorker(store, runtime, NewMemoryQueue())
	if err := worker.handle(container.Name); err != nil {
		t.Fatalf("unexpected handle error: %v", err)
	}

	found, err := runtime.Inspect(container.Name)
	if err != nil {
		t.Fatalf("unexpected runtime inspect error: %v", err)
	}
	if found {
		t.Error("container found in runtime after deletion")
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

	found, err := runtime.Inspect(container.Name)
	if err != nil {
		t.Fatalf("unexpected runtime inspect error: %v", err)
	}
	if !found {
		t.Error("container missing from runtime after handling")
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
				if err := runtime.Create(tt.container.Name); err != nil {
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
			found, err := runtime.Inspect(tt.container.Name)
			if err != nil {
				t.Fatalf("unexpected runtime inspect error: %v", err)
			}
			if found != tt.wantRuntime {
				t.Errorf(
					"container %q runtime existence: got %t, want %t",
					tt.container.Name,
					found,
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
