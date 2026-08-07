package container

import (
	"errors"
	"testing"
	"time"
)

func TestServiceCreate(t *testing.T) {
	store := NewMemoryStore()
	enqueueCalls := 0
	var wantEnqueued Container
	enqueue := enqueuerFunc(func(name string) {
		enqueueCalls++

		if name != wantEnqueued.Name {
			t.Errorf("enqueued container name %q, want %q", name, wantEnqueued.Name)
		}

		stored, err := store.Get(name)
		if err != nil {
			t.Errorf("container not stored before enqueue: %v", err)
			return
		}
		if stored != wantEnqueued {
			t.Errorf("got stored container %+v before enqueue, want %+v", stored, wantEnqueued)
		}
	})

	startTime := time.Date(2026, time.July, 19, 15, 0, 0, 0, time.UTC)
	clock := newFakeClock(startTime, time.Minute)

	s := NewService(store, enqueue, WithClock(clock.now))

	tests := []struct {
		container Container
		wantTime  time.Time
	}{
		{
			container: Container{
				Name:  "hello",
				Image: "docker.io/library/alpine:latest",
			},
			wantTime: startTime,
		},
		{
			container: Container{
				Name:  "database",
				Image: "docker.io/library/postgres:latest",
			},
			wantTime: startTime.Add(time.Minute),
		},
	}

	for _, tt := range tests {
		want := tt.container
		want.State = StateCreating
		want.CreatedAt = tt.wantTime
		wantEnqueued = want

		got, err := s.Create(tt.container)
		if err != nil {
			t.Fatalf("unexpected service create error: %v", err)
		}

		if got != want {
			t.Errorf("got %+v, want %+v", got, want)
		}
	}

	// Ensure the clock is called exactly once per service create call.
	if clock.calls != len(tests) {
		t.Errorf("clock called %d times, want %d", clock.calls, len(tests))
	}
	// Ensure the enqueuer is called exactly once per service create call.
	if enqueueCalls != len(tests) {
		t.Errorf("enqueue called %d times, want %d", enqueueCalls, len(tests))
	}
}

func TestServiceCreateDoesNotEnqueueWhenStoreFails(t *testing.T) {
	wantErr := errors.New("store unavailable")
	store := failingStore{err: wantErr}
	enqueued := false
	enqueue := enqueuerFunc(func(string) {
		enqueued = true
	})
	s := NewService(store, enqueue)

	_, err := s.Create(Container{
		Name:  "hello",
		Image: "docker.io/library/alpine:latest",
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("got error %v, want %v", err, wantErr)
	}

	if enqueued {
		t.Errorf("container enqueued after store failure")
	}
}

func TestServiceGet(t *testing.T) {
	store := NewMemoryStore()
	container := Container{
		Name:  "hello",
		Image: "docker.io/library/alpine:latest",
		State: StateCreating,
	}
	if err := store.Create(container); err != nil {
		t.Fatalf("unexpected store create error: %v", err)
	}

	enqueue := enqueuerFunc(func(string) {})
	service := NewService(store, enqueue)

	got, err := service.Get(container.Name)
	if err != nil {
		t.Fatalf("unexpected service get error: %v", err)
	}
	if got != container {
		t.Errorf("got %+v, want %+v", got, container)
	}
}

func TestServiceStart(t *testing.T) {
	store := NewMemoryStore()
	container := Container{
		Name:  "hello",
		Image: "docker.io/library/alpine:latest",
		State: StateCreated,
	}
	if err := store.Create(container); err != nil {
		t.Fatalf("unexpected store create error: %v", err)
	}

	var enqueuedName string
	enqueue := enqueuerFunc(func(name string) {
		enqueuedName = name
	})
	service := NewService(store, enqueue)

	got, err := service.Start(container.Name)
	if err != nil {
		t.Fatalf("unexpected service start error: %v", err)
	}

	want := container
	want.State = StateStarting
	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}

	stored, err := store.Get(container.Name)
	if err != nil {
		t.Fatalf("get stored container: %v", err)
	}
	if stored != want {
		t.Errorf("got stored container %+v, want %+v", stored, want)
	}

	if enqueuedName != container.Name {
		t.Errorf("enqueued %q, want %q", enqueuedName, container.Name)
	}
}

func TestServiceStartStoresContainerBeforeEnqueueing(t *testing.T) {
	store := NewMemoryStore()
	container := Container{
		Name:  "hello",
		Image: "docker.io/library/alpine:latest",
		State: StateCreated,
	}
	if err := store.Create(container); err != nil {
		t.Fatalf("unexpected store create error: %v", err)
	}

	var enqueuedName string
	enqueue := enqueuerFunc(func(name string) {
		enqueuedName = name

		stored, err := store.Get(name)
		if err != nil {
			t.Errorf("container not stored before enqueue: %v", err)
			return
		}
		if stored.State != StateStarting {
			t.Errorf("got state %q when enqueued, want %q", stored.State, StateStarting)
		}
	})
	service := NewService(store, enqueue)

	if _, err := service.Start(container.Name); err != nil {
		t.Fatalf("unexpected service start error: %v", err)
	}
	if enqueuedName != container.Name {
		t.Errorf("enqueued %q, want %q", enqueuedName, container.Name)
	}
}

func TestServiceStartRejectsInvalidState(t *testing.T) {
	for _, state := range []State{
		StateCreating,
		StateStarting,
		StateRunning,
		StateStopping,
		StateStopped,
		StateDeleting,
	} {
		t.Run(string(state), func(t *testing.T) {
			store := NewMemoryStore()
			container := Container{
				Name:  "hello",
				Image: "docker.io/library/alpine:latest",
				State: state,
			}
			if err := store.Create(container); err != nil {
				t.Fatalf("unexpected store create error: %v", err)
			}

			enqueued := false
			enqueue := enqueuerFunc(func(string) {
				enqueued = true
			})
			service := NewService(store, enqueue)

			_, err := service.Start(container.Name)
			if !errors.Is(err, ErrOperationNotAllowed) {
				t.Fatalf("got error %v, want %v", err, ErrOperationNotAllowed)
			}

			stored, err := store.Get(container.Name)
			if err != nil {
				t.Fatalf("get stored container: %v", err)
			}
			if stored != container {
				t.Errorf("got stored container %+v, want unchanged %+v", stored, container)
			}
			if enqueued {
				t.Errorf("container in state %q was enqueued for start", state)
			}
		})
	}
}

func TestServiceStartRejectsDeletionRequested(t *testing.T) {
	store := NewMemoryStore()
	container := Container{
		Name:              "hello",
		Image:             "docker.io/library/alpine:latest",
		State:             StateCreated,
		DeletionTimestamp: time.Date(2026, time.July, 19, 15, 0, 0, 0, time.UTC),
	}
	if err := store.Create(container); err != nil {
		t.Fatalf("unexpected store create error: %v", err)
	}

	enqueued := false
	enqueue := enqueuerFunc(func(string) {
		enqueued = true
	})
	service := NewService(store, enqueue)

	_, err := service.Start(container.Name)
	if !errors.Is(err, ErrOperationNotAllowed) {
		t.Fatalf("got error %v, want %v", err, ErrOperationNotAllowed)
	}

	stored, err := store.Get(container.Name)
	if err != nil {
		t.Fatalf("get stored container: %v", err)
	}
	if stored != container {
		t.Errorf("got stored container %+v, want unchanged %+v", stored, container)
	}
	if enqueued {
		t.Error("container marked for deletion was enqueued for start")
	}
}

func TestServiceStop(t *testing.T) {
	store := NewMemoryStore()
	container := Container{
		Name:  "hello",
		Image: "docker.io/library/alpine:latest",
		State: StateRunning,
	}
	if err := store.Create(container); err != nil {
		t.Fatalf("unexpected store create error: %v", err)
	}

	var enqueuedName string
	enqueue := enqueuerFunc(func(name string) {
		enqueuedName = name
	})
	service := NewService(store, enqueue)

	got, err := service.Stop(container.Name)
	if err != nil {
		t.Fatalf("unexpected service stop error: %v", err)
	}

	want := container
	want.State = StateStopping
	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}

	stored, err := store.Get(container.Name)
	if err != nil {
		t.Fatalf("get stored container: %v", err)
	}
	if stored != want {
		t.Errorf("got stored container %+v, want %+v", stored, want)
	}

	if enqueuedName != container.Name {
		t.Errorf("enqueued %q, want %q", enqueuedName, container.Name)
	}
}

func TestServiceStopStoresContainerBeforeEnqueueing(t *testing.T) {
	store := NewMemoryStore()
	container := Container{
		Name:  "hello",
		Image: "docker.io/library/alpine:latest",
		State: StateRunning,
	}
	if err := store.Create(container); err != nil {
		t.Fatalf("unexpected store create error: %v", err)
	}

	var enqueuedName string
	enqueue := enqueuerFunc(func(name string) {
		enqueuedName = name

		stored, err := store.Get(name)
		if err != nil {
			t.Errorf("container not stored before enqueue: %v", err)
			return
		}
		if stored.State != StateStopping {
			t.Errorf("got state %q when enqueued, want %q", stored.State, StateStopping)
		}
	})
	service := NewService(store, enqueue)

	if _, err := service.Stop(container.Name); err != nil {
		t.Fatalf("unexpected service stop error: %v", err)
	}
	if enqueuedName != container.Name {
		t.Errorf("enqueued %q, want %q", enqueuedName, container.Name)
	}
}

func TestServiceStopRejectsInvalidState(t *testing.T) {
	for _, state := range []State{
		StateCreating,
		StateCreated,
		StateStarting,
		StateStopping,
		StateStopped,
		StateDeleting,
	} {
		t.Run(string(state), func(t *testing.T) {
			store := NewMemoryStore()
			container := Container{
				Name:  "hello",
				Image: "docker.io/library/alpine:latest",
				State: state,
			}
			if err := store.Create(container); err != nil {
				t.Fatalf("unexpected store create error: %v", err)
			}

			enqueued := false
			enqueue := enqueuerFunc(func(string) {
				enqueued = true
			})
			service := NewService(store, enqueue)

			_, err := service.Stop(container.Name)
			if !errors.Is(err, ErrOperationNotAllowed) {
				t.Fatalf("got error %v, want %v", err, ErrOperationNotAllowed)
			}

			stored, err := store.Get(container.Name)
			if err != nil {
				t.Fatalf("get stored container: %v", err)
			}
			if stored != container {
				t.Errorf("got stored container %+v, want unchanged %+v", stored, container)
			}
			if enqueued {
				t.Errorf("container in state %q was enqueued for stop", state)
			}
		})
	}
}

func TestServiceStopRejectsDeletionRequested(t *testing.T) {
	store := NewMemoryStore()
	container := Container{
		Name:              "hello",
		Image:             "docker.io/library/alpine:latest",
		State:             StateRunning,
		DeletionTimestamp: time.Date(2026, time.July, 19, 15, 0, 0, 0, time.UTC),
	}
	if err := store.Create(container); err != nil {
		t.Fatalf("unexpected store create error: %v", err)
	}

	enqueued := false
	enqueue := enqueuerFunc(func(string) {
		enqueued = true
	})
	service := NewService(store, enqueue)

	_, err := service.Stop(container.Name)
	if !errors.Is(err, ErrOperationNotAllowed) {
		t.Fatalf("got error %v, want %v", err, ErrOperationNotAllowed)
	}

	stored, err := store.Get(container.Name)
	if err != nil {
		t.Fatalf("get stored container: %v", err)
	}
	if stored != container {
		t.Errorf("got stored container %+v, want unchanged %+v", stored, container)
	}
	if enqueued {
		t.Error("container marked for deletion was enqueued for stop")
	}
}

func TestServiceDelete(t *testing.T) {
	for _, state := range []State{StateCreating, StateCreated, StateStopped} {
		t.Run(string(state), func(t *testing.T) {
			store := NewMemoryStore()
			container := Container{
				Name:  "hello",
				Image: "docker.io/library/alpine:latest",
				State: state,
			}
			if err := store.Create(container); err != nil {
				t.Fatalf("unexpected store create error: %v", err)
			}

			var enqueuedName string
			enqueue := enqueuerFunc(func(name string) {
				enqueuedName = name
			})
			fixedTime := time.Date(2026, time.July, 19, 15, 0, 0, 0, time.UTC)
			clock := newFakeClock(fixedTime, 0)

			service := NewService(store, enqueue, WithClock(clock.now))

			want := container
			want.DeletionTimestamp = fixedTime

			got, err := service.Delete(container.Name)
			if err != nil {
				t.Fatalf("unexpected service delete error: %v", err)
			}
			if got != want {
				t.Errorf("got %+v, want %+v", got, want)
			}

			stored, err := store.Get(container.Name)
			if err != nil {
				t.Fatalf("get stored container: %v", err)
			}
			if stored != want {
				t.Errorf("got stored container %+v, want %+v", stored, want)
			}

			if enqueuedName != container.Name {
				t.Errorf("enqueued %q, want %q", enqueuedName, container.Name)
			}
			if clock.calls != 1 {
				t.Errorf("clock called %d times, want 1", clock.calls)
			}
		})
	}
}

func TestServiceDeleteRejectsInvalidState(t *testing.T) {
	for _, state := range []State{StateStarting, StateRunning, StateStopping} {
		t.Run(string(state), func(t *testing.T) {
			store := NewMemoryStore()
			container := Container{
				Name:  "hello",
				Image: "docker.io/library/alpine:latest",
				State: state,
			}
			if err := store.Create(container); err != nil {
				t.Fatalf("unexpected store create error: %v", err)
			}

			enqueued := false
			enqueue := enqueuerFunc(func(string) {
				enqueued = true
			})
			service := NewService(store, enqueue)

			_, err := service.Delete(container.Name)
			if !errors.Is(err, ErrOperationNotAllowed) {
				t.Fatalf("got error %v, want %v", err, ErrOperationNotAllowed)
			}

			stored, err := store.Get(container.Name)
			if err != nil {
				t.Fatalf("get stored container: %v", err)
			}
			if stored != container {
				t.Errorf("got stored container %+v, want unchanged %+v", stored, container)
			}
			if enqueued {
				t.Errorf("container in state %q was enqueued for deletion", state)
			}
		})
	}
}

func TestServiceDeleteRejectsAlreadyRequestedDeletion(t *testing.T) {
	deletionTime := time.Date(2026, time.July, 19, 15, 0, 0, 0, time.UTC)
	container := Container{
		Name:              "hello",
		Image:             "docker.io/library/alpine:latest",
		State:             StateCreated,
		DeletionTimestamp: deletionTime,
	}
	store := NewMemoryStore()
	if err := store.Create(container); err != nil {
		t.Fatalf("unexpected store create error: %v", err)
	}

	enqueued := false
	enqueue := enqueuerFunc(func(name string) {
		enqueued = true
	})
	clock := newFakeClock(deletionTime.Add(time.Minute), 0)
	service := NewService(store, enqueue, WithClock(clock.now))

	_, err := service.Delete(container.Name)
	if !errors.Is(err, ErrOperationNotAllowed) {
		t.Fatalf("got %v error, want %v error", err, ErrOperationNotAllowed)
	}

	stored, err := store.Get(container.Name)
	if err != nil {
		t.Fatalf("get stored container: %v", err)
	}
	if stored != container {
		t.Errorf("got stored container %+v, want unchanged %+v", stored, container)
	}

	if enqueued {
		t.Errorf("container already set for deletion enqueued")
	}
	if clock.calls != 0 {
		t.Errorf("clock called %d times, want 0", clock.calls)
	}
}

func TestServiceDeleteStoresContainerBeforeEnqueueing(t *testing.T) {
	store := NewMemoryStore()
	container := Container{
		Name:  "hello",
		Image: "docker.io/library/alpine:latest",
		State: StateCreated,
	}
	if err := store.Create(container); err != nil {
		t.Fatalf("unexpected store create error: %v", err)
	}

	fixedTime := time.Date(2026, time.July, 19, 15, 0, 0, 0, time.UTC)
	var enqueuedName string
	enqueue := enqueuerFunc(func(name string) {
		enqueuedName = name

		stored, err := store.Get(name)
		if err != nil {
			t.Errorf("container not stored before enqueue: %v", err)
			return
		}
		if stored.DeletionTimestamp != fixedTime {
			t.Errorf(
				"got deletion timestamp %v when enqueued, want %v",
				stored.DeletionTimestamp,
				fixedTime,
			)
		}
	})
	service := NewService(
		store,
		enqueue,
		WithClock(func() time.Time { return fixedTime }),
	)

	if _, err := service.Delete(container.Name); err != nil {
		t.Fatalf("unexpected service delete error: %v", err)
	}
	if enqueuedName != container.Name {
		t.Errorf("enqueued %q, want %q", enqueuedName, container.Name)
	}
}
