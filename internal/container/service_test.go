package container

import (
	"errors"
	"testing"
	"time"
)

// the pattern used on enqueuerFunc is like the handlerFunc of net/http

type enqueuerFunc func(name string)

func (f enqueuerFunc) add(name string) {
	f(name)
}

type failingStore struct {
	err error
}

func (s failingStore) Create(Container) error {
	return s.err
}

func (s failingStore) Get(string) (Container, error) {
	panic("unexpected call to Store.Get")
}

func (s failingStore) Modify(string, func(*Container)) error {
	panic("unexpected call to Store.Modify")
}

type fakeClock struct {
	current time.Time
	advance time.Duration
	calls   int
}

func newFakeClock(start time.Time, advance time.Duration) *fakeClock {
	return &fakeClock{
		current: start,
		advance: advance,
	}
}

// now is not safe to call concurrently.
func (clock *fakeClock) now() time.Time {
	now := clock.current
	clock.current = clock.current.Add(clock.advance)
	clock.calls++
	return now
}

func TestServiceCreate(t *testing.T) {
	store := NewMemoryStore()
	enqueue := enqueuerFunc(func(string) {})

	startTime := time.Date(2026, time.July, 19, 15, 0, 0, 0, time.UTC)
	clock := newFakeClock(startTime, 0)

	s := newService(store, enqueue, withClock(clock.now))

	container := Container{
		Name:  "hello",
		Image: "docker.io/library/alpine:latest",
	}

	got, err := s.create(container)
	if err != nil {
		t.Fatalf("unexpected service create error: %v", err)
	}

	want := container
	want.State = StateCreating
	want.CreatedAt = startTime

	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}

	c, err := store.Get(want.Name)
	if err != nil {
		t.Fatalf("unexpected store get error: %v", err)
	}
	if c != want {
		t.Errorf("got store %+v, want %+v", c, want)
	}

	// Ensure the clock is called exactly once per service create call.
	if clock.calls != 1 {
		t.Errorf("clock called %d times, want %d", clock.calls, 1)
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

	s := newService(store, enqueue)

	container := Container{
		Name:  "hello",
		Image: "docker.io/library/alpine:latest",
	}

	if _, err := s.create(container); err != nil {
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
	s := newService(store, enqueue)

	_, err := s.create(Container{
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
