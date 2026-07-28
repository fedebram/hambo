package container

import (
	"errors"
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

	err := store.Modify("hello", func(container *Container) {
		container.State = StateCreated
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

	err := store.Modify("hello", func(container *Container) {
		container.State = StateCreated
	})
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("got %v error, want %v error", err, ErrNotFound)
	}

	_, err = store.Get("hello")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("container was stored after failed modify: %v", err)
	}
}
