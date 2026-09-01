package containertest

import (
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/fedebram/hambo/internal/container"
)

func TestStore(t *testing.T, newStore func(*testing.T) container.Store) {
	t.Helper()

	t.Run("created container can be retrieved", func(t *testing.T) {
		store := newStore(t)
		want := container.Container{
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
	})

	t.Run("getting missing container returns not found", func(t *testing.T) {
		store := newStore(t)

		_, err := store.Get("missing")
		if !errors.Is(err, container.ErrNotFound) {
			t.Errorf("got error %v, want %v", err, container.ErrNotFound)
		}
	})

	t.Run("duplicate container is rejected", func(t *testing.T) {
		store := newStore(t)
		original := container.Container{
			Name: "hello",
			CreatedAt: time.Date(
				2026, time.July, 19, 15, 0, 0, 0, time.UTC,
			),
		}
		replacement := container.Container{
			Name: "hello",
			CreatedAt: time.Date(
				2026, time.July, 20, 15, 0, 0, 0, time.UTC,
			),
		}

		if err := store.Create(original); err != nil {
			t.Fatalf("could not create container: %v", err)
		}
		if err := store.Create(replacement); !errors.Is(err, container.ErrAlreadyExists) {
			t.Fatalf("got error %v, want %v", err, container.ErrAlreadyExists)
		}

		got, err := store.Get("hello")
		if err != nil {
			t.Fatalf("could not get container: %v", err)
		}
		if got != original {
			t.Errorf("got %+v, want unchanged %+v", got, original)
		}
	})

	t.Run("container can be modified", func(t *testing.T) {
		store := newStore(t)
		original := container.Container{
			Name:  "hello",
			Image: "docker.io/library/alpine:latest",
			State: container.StateCreating,
		}
		if err := store.Create(original); err != nil {
			t.Fatalf("unexpected create error: %v", err)
		}

		err := store.Modify("hello", func(c *container.Container) error {
			c.State = container.StateCreated
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
		want.State = container.StateCreated
		if got != want {
			t.Errorf("got %+v, want %+v", got, want)
		}
	})

	t.Run("modifying missing container returns not found", func(t *testing.T) {
		store := newStore(t)

		err := store.Modify("hello", func(c *container.Container) error {
			c.State = container.StateCreated
			return nil
		})
		if !errors.Is(err, container.ErrNotFound) {
			t.Errorf("got error %v, want %v", err, container.ErrNotFound)
		}

		_, err = store.Get("hello")
		if !errors.Is(err, container.ErrNotFound) {
			t.Errorf("container was stored after failed modify: %v", err)
		}
	})

	t.Run("failed modification is not stored", func(t *testing.T) {
		store := newStore(t)
		original := container.Container{
			Name:  "hello",
			State: container.StateCreated,
		}
		if err := store.Create(original); err != nil {
			t.Fatalf("create container: %v", err)
		}

		wantErr := errors.New("callback failed")
		err := store.Modify("hello", func(c *container.Container) error {
			c.State = container.StateRunning
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
	})

	t.Run("container name cannot be modified", func(t *testing.T) {
		store := newStore(t)
		original := container.Container{
			Name:  "hello",
			Image: "docker.io/library/alpine:latest",
			State: container.StateCreated,
		}
		if err := store.Create(original); err != nil {
			t.Fatalf("create container: %v", err)
		}

		err := store.Modify("hello", func(c *container.Container) error {
			c.Name = "goodbye"
			return nil
		})
		if !errors.Is(err, container.ErrOperationNotAllowed) {
			t.Fatalf("got error %v, want %v", err, container.ErrOperationNotAllowed)
		}

		got, err := store.Get("hello")
		if err != nil {
			t.Fatalf("get original container: %v", err)
		}
		if got != original {
			t.Fatalf("got %+v, want unchanged %+v", got, original)
		}

		_, err = store.Get("goodbye")
		if !errors.Is(err, container.ErrNotFound) {
			t.Fatalf("renamed container was stored: %v", err)
		}
	})

	t.Run("container can be deleted", func(t *testing.T) {
		store := newStore(t)
		c := container.Container{
			Name:  "hello",
			Image: "docker.io/library/alpine:latest",
			State: container.StateCreating,
		}
		if err := store.Create(c); err != nil {
			t.Fatalf("unexpected create error: %v", err)
		}

		if err := store.Delete("hello"); err != nil {
			t.Fatalf("unexpected delete error: %v", err)
		}
		_, err := store.Get("hello")
		if !errors.Is(err, container.ErrNotFound) {
			t.Fatalf("got error %v, want %v", err, container.ErrNotFound)
		}
	})

	t.Run("deleting missing container succeeds", func(t *testing.T) {
		store := newStore(t)
		if err := store.Delete("missing"); err != nil {
			t.Fatalf("unexpected delete error: %v", err)
		}
	})

	t.Run("containers can be listed", func(t *testing.T) {
		store := newStore(t)

		cs, err := store.List()
		if err != nil {
			t.Fatalf("unexpected list error: %v", err)
		}
		if len(cs) != 0 {
			t.Fatalf("expected empty list, got %d containers", len(cs))
		}

		const count = 100
		want := make([]container.Container, count)
		errCh := make(chan error, count)

		var wg sync.WaitGroup
		for i := range count {
			want[i] = container.Container{
				Name:  fmt.Sprintf("container-%03d", i),
				Image: fmt.Sprintf("example.com/image:%d", i),
				State: container.StateCreated,
			}

			wg.Add(1)
			go func(c container.Container) {
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
		for i := range want {
			if cs[i] != want[i] {
				t.Fatalf("container %d: got %+v, want %+v", i, cs[i], want[i])
			}
		}
	})
}
