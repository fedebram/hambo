package container

import (
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestMemoryStoreCreateAndGet(t *testing.T) {
	store := NewMemoryStore()

	want := Container{
		Name: "hello",
		CreatedAt: time.Date(
			2026, time.July, 19, 15, 0, 0, 0, time.UTC,
		),
	}

	if err := store.Create(want); err != nil {
		t.Fatalf("could not create container: %v", err)
	}

	got, err := store.Get("hello")
	if err != nil {
		t.Fatalf("could not get container: %v", err)
	}

	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestMemoryStoreGetMissing(t *testing.T) {
	store := NewMemoryStore()

	_, err := store.Get("missing")

	if !errors.Is(err, ErrNotFound) {
		t.Errorf("got error %v, want %v", err, ErrNotFound)
	}
}

func TestMemoryStoreCreateRejectsDuplicate(t *testing.T) {
	store := NewMemoryStore()

	original := Container{
		Name: "hello",
		CreatedAt: time.Date(
			2026, time.July, 19, 15, 0, 0, 0, time.UTC,
		),
	}
	replacement := Container{
		Name: "hello",
		CreatedAt: time.Date(
			2026, time.July, 20, 15, 0, 0, 0, time.UTC,
		),
	}

	if err := store.Create(original); err != nil {
		t.Fatalf("could not create container: %v", err)
	}

	err := store.Create(replacement)
	if err == nil {
		t.Fatalf("expected container already exists error got nil error")
	}
	if !errors.Is(err, ErrAlreadyExists) {
		t.Fatalf("expected container already exists error got %v", err)
	}

	got, err := store.Get("hello")
	if err != nil {
		t.Fatalf("could not get container: %v", err)
	}

	if got != original {
		t.Errorf("got %+v, want %+v", got, original)
	}
}

func TestMemoryStoreModify(t *testing.T) {
	store := NewMemoryStore()

	original := Container{
		Name:  "hello",
		Image: "docker.io/library/alpine:latest",
		State: StateCreating,
	}
	if err := store.Create(original); err != nil {
		t.Fatalf("unexpected create error: %v", err)
	}

	err := store.Modify("hello", func(container *Container) error {
		container.State = StateCreated
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected modify error: %v", err)
	}

	got, err := store.Get("hello")
	if err != nil {
		t.Fatalf("unexpected get error: %v", err)
	}

	want := original
	want.State = StateCreated

	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestMemoryStoreModifyNotFound(t *testing.T) {
	store := NewMemoryStore()

	err := store.Modify("hello", func(container *Container) error {
		container.State = StateCreated
		return nil
	})
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("got %v error, want %v error", err, ErrNotFound)
	}

	_, err = store.Get("hello")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("container was stored after failed modify: %v", err)
	}
}

func TestMemoryStoreModifyDoesNotStoreChangesWhenCallbackFails(t *testing.T) {
	store := NewMemoryStore()

	original := Container{
		Name:  "hello",
		State: StateCreated,
	}

	if err := store.Create(original); err != nil {
		t.Fatalf("create container: %v", err)
	}

	wantErr := errors.New("callback failed")

	// this is pretty useful because we can check pure business logic inside the "transaction"
	// and return an error if something is not good.
	// the same error gets returned by modify!
	err := store.Modify("hello", func(c *Container) error {
		c.State = StateRunning
		return wantErr
	})

	if !errors.Is(err, wantErr) {
		t.Fatalf("got error %v, want %v", err, wantErr)
	}

	got, err := store.Get("hello")
	if err != nil {
		t.Fatalf("get container: %v", err)
	}

	if got != original {
		t.Fatalf("got %+v, want unchanged %+v", got, original)
	}
}

func TestMemoryStoreDelete(t *testing.T) {
	store := NewMemoryStore()

	c := Container{
		Name:  "hello",
		Image: "docker.io/library/alpine:latest",
		State: StateCreating,
	}
	if err := store.Create(c); err != nil {
		t.Fatalf("unexpected create error: %v", err)
	}

	if err := store.Delete("hello"); err != nil {
		t.Fatalf("unexpected delete error: %v", err)
	}

	_, err := store.Get("hello")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("got %v error, want %v error", err, ErrNotFound)
	}
}

func TestMemoryStoreDeleteMissingIsNoOp(t *testing.T) {
	store := NewMemoryStore()

	if err := store.Delete("missing"); err != nil {
		t.Fatalf("unexpected delete error: %v", err)
	}
}

func TestMemoryStoreList(t *testing.T) {
	store := NewMemoryStore()

	cs, err := store.List()
	if err != nil {
		t.Fatalf("unexpected list error: %v", err)
	}

	if len(cs) != 0 {
		t.Fatalf("expected empty list, got %d containers", len(cs))
	}

	const count = 100
	want := make([]Container, count)
	errCh := make(chan error, count)

	var wg sync.WaitGroup
	for i := range count {
		want[i] = Container{
			Name:  fmt.Sprintf("container-%03d", i),
			Image: fmt.Sprintf("example.com/image:%d", i),
			State: StateCreated,
		}

		wg.Add(1)
		go func(c Container) {
			defer wg.Done()
			if err := store.Create(c); err != nil {
				errCh <- err
			}
		}(want[i])
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		t.Fatalf("create container: %v", err)
	}

	cs, err = store.List()
	if err != nil {
		t.Fatalf("unexpected list error: %v", err)
	}

	if len(cs) != len(want) {
		t.Fatalf("got %d containers, want %d", len(cs), len(want))
	}

	// this way we check also the ordering
	for i := range want {
		if cs[i] != want[i] {
			// exit early with fatalf
			t.Fatalf("container %d: got %+v, want %+v", i, cs[i], want[i])
		}
	}
}
