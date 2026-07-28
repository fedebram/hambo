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
