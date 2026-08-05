package container

import (
	"errors"
	"testing"
	"time"
)

func TestServiceCreate(t *testing.T) {
	store := NewMemoryStore()
	enqueueCalls := 0
	enqueue := enqueuerFunc(func(string) {
		enqueueCalls++
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
		got, err := s.Create(tt.container)
		if err != nil {
			t.Fatalf("unexpected service create error: %v", err)
		}

		want := tt.container
		want.State = StateCreating
		want.CreatedAt = tt.wantTime

		if got != want {
			t.Errorf("got %+v, want %+v", got, want)
		}

		stored, err := store.Get(want.Name)
		if err != nil {
			t.Fatalf("unexpected store get error: %v", err)
		}
		if stored != want {
			t.Errorf("got stored container %+v, want %+v", stored, want)
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

func TestServiceCreateStoresContainerBeforeEnqueuing(t *testing.T) {
	store := NewMemoryStore()
	var gotName string
	enqueue := enqueuerFunc(func(name string) {
		gotName = name

		stored, err := store.Get(name)
		if err != nil {
			t.Errorf("container not stored before enqueue: %v", err)
			return
		}
		if stored.State != StateCreating {
			t.Errorf("got state %q when enqueued, want %q", stored.State, StateCreating)
		}
	})

	s := NewService(store, enqueue)

	container := Container{
		Name:  "hello",
		Image: "docker.io/library/alpine:latest",
	}

	if _, err := s.Create(container); err != nil {
		t.Fatalf("unexpected service create error %v", err)
	}

	wantName := container.Name
	if gotName != wantName {
		t.Errorf("enqueued container name %q, want %q", gotName, wantName)
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

func TestServiceDelete(t *testing.T) {
	for _, state := range []State{StateCreating, StateCreated} {
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

func TestServiceDeleteRejectsRunningContainer(t *testing.T) {
	store := NewMemoryStore()
	container := Container{
		Name:  "hello",
		Image: "docker.io/library/alpine:latest",
		State: StateRunning,
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
		t.Error("running container was enqueued for deletion")
	}
}

func TestServiceDeleteNotFound(t *testing.T) {
	store := NewMemoryStore()
	enqueued := false
	enqueue := enqueuerFunc(func(string) {
		enqueued = true
	})
	clock := newFakeClock(time.Time{}, 0)
	service := NewService(store, enqueue, WithClock(clock.now))

	_, err := service.Delete("missing")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("got error %v, want %v", err, ErrNotFound)
	}

	if enqueued {
		t.Error("missing container was enqueued for deletion")
	}
	if clock.calls != 0 {
		t.Errorf("clock called %d times, want 0", clock.calls)
	}
}

func TestServiceDeletePreservesDeletionTimestamp(t *testing.T) {
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

	var enqueuedName string
	enqueue := enqueuerFunc(func(name string) {
		enqueuedName = name
	})
	clock := newFakeClock(deletionTime.Add(time.Minute), 0)
	service := NewService(store, enqueue, WithClock(clock.now))

	got, err := service.Delete(container.Name)
	if err != nil {
		t.Fatalf("unexpected service delete error: %v", err)
	}
	if got != container {
		t.Errorf("got %+v, want unchanged %+v", got, container)
	}

	stored, err := store.Get(container.Name)
	if err != nil {
		t.Fatalf("get stored container: %v", err)
	}
	if stored != container {
		t.Errorf("got stored container %+v, want unchanged %+v", stored, container)
	}

	if enqueuedName != container.Name {
		t.Errorf("enqueued %q, want %q", enqueuedName, container.Name)
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
