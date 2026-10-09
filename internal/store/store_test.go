package store

import (
	"errors"
	"fmt"
	"sync"
	"testing"
)

func TestStore(t *testing.T) {
	t.Run("created entry can be retrieved", func(t *testing.T) {
		store := newTestStore(t)
		want := "world"

		if err := store.Create("hello", want); err != nil {
			t.Fatalf("create entry: %v", err)
		}

		got, err := store.Get("hello")
		if err != nil {
			t.Fatalf("get entry: %v", err)
		}
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("getting missing entry returns not found", func(t *testing.T) {
		store := newTestStore(t)

		_, err := store.Get("missing")
		if !errors.Is(err, ErrNotFound) {
			t.Errorf("got error %v, want %v", err, ErrNotFound)
		}
	})

	t.Run("duplicate entry is rejected", func(t *testing.T) {
		store := newTestStore(t)
		original := "world"

		if err := store.Create("hello", original); err != nil {
			t.Fatalf("create entry: %v", err)
		}
		if err := store.Create("hello", "replacement"); !errors.Is(err, ErrAlreadyExists) {
			t.Fatalf("got error %v, want %v", err, ErrAlreadyExists)
		}

		got, err := store.Get("hello")
		if err != nil {
			t.Fatalf("get entry: %v", err)
		}
		if got != original {
			t.Errorf("got %q, want unchanged %q", got, original)
		}
	})

	t.Run("entry can be modified", func(t *testing.T) {
		store := newTestStore(t)

		if err := store.Create("hello", "world"); err != nil {
			t.Fatalf("create entry: %v", err)
		}

		err := store.Modify("hello", func(value string) (string, error) {
			return value + "!", nil
		})
		if err != nil {
			t.Fatalf("modify entry: %v", err)
		}

		got, err := store.Get("hello")
		if err != nil {
			t.Fatalf("get entry: %v", err)
		}
		if want := "world!"; got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("modifying missing entry returns not found", func(t *testing.T) {
		store := newTestStore(t)
		called := false

		err := store.Modify("missing", func(value string) (string, error) {
			called = true
			return "updated", nil
		})
		if !errors.Is(err, ErrNotFound) {
			t.Fatalf("got error %v, want %v", err, ErrNotFound)
		}
		if called {
			t.Error("callback was called for a missing entry")
		}

		_, err = store.Get("missing")
		if !errors.Is(err, ErrNotFound) {
			t.Errorf("got error %v after failed modify, want %v", err, ErrNotFound)
		}
	})

	t.Run("failed modification is not stored", func(t *testing.T) {
		store := newTestStore(t)
		original := "world"

		if err := store.Create("hello", original); err != nil {
			t.Fatalf("create entry: %v", err)
		}

		wantErr := errors.New("callback failed")
		err := store.Modify("hello", func(value string) (string, error) {
			return "replacement", wantErr
		})
		if !errors.Is(err, wantErr) {
			t.Fatalf("got error %v, want %v", err, wantErr)
		}

		got, err := store.Get("hello")
		if err != nil {
			t.Fatalf("get entry: %v", err)
		}
		if got != original {
			t.Errorf("got %q, want unchanged %q", got, original)
		}
	})

	t.Run("entry can be deleted", func(t *testing.T) {
		store := newTestStore(t)

		if err := store.Create("hello", "world"); err != nil {
			t.Fatalf("create entry: %v", err)
		}
		if err := store.Delete("hello"); err != nil {
			t.Fatalf("delete entry: %v", err)
		}

		_, err := store.Get("hello")
		if !errors.Is(err, ErrNotFound) {
			t.Errorf("got error %v, want %v", err, ErrNotFound)
		}
	})

	t.Run("deleting missing entry succeeds", func(t *testing.T) {
		store := newTestStore(t)

		if err := store.Delete("missing"); err != nil {
			t.Fatalf("delete missing entry: %v", err)
		}
	})

	t.Run("entries can be listed in key order", func(t *testing.T) {
		store := newTestStore(t)

		for _, entry := range []Entry{
			{Key: "charlie", Value: "three"},
			{Key: "alpha", Value: "one"},
			{Key: "bravo", Value: "two"},
		} {
			if err := store.Create(entry.Key, entry.Value); err != nil {
				t.Fatalf("create entry: %v", err)
			}
		}

		got, err := store.List()
		if err != nil {
			t.Fatalf("list entries: %v", err)
		}

		want := []Entry{
			{Key: "alpha", Value: "one"},
			{Key: "bravo", Value: "two"},
			{Key: "charlie", Value: "three"},
		}
		if len(got) != len(want) {
			t.Fatalf("got %d entries, want %d", len(got), len(want))
		}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("entry %d: got %+v, want %+v", i, got[i], want[i])
			}
		}
	})

	t.Run("empty store lists no entries", func(t *testing.T) {
		store := newTestStore(t)

		got, err := store.List()
		if err != nil {
			t.Fatalf("list entries: %v", err)
		}
		if len(got) != 0 {
			t.Errorf("got %d entries, want 0", len(got))
		}
	})

	t.Run("entries can be created concurrently", func(t *testing.T) {
		store := newTestStore(t)
		const count = 100

		errCh := make(chan error, count)
		var wg sync.WaitGroup

		for i := range count {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()

				key := fmt.Sprintf("entry-%03d", i)
				value := fmt.Sprintf("value-%d", i)
				if err := store.Create(key, value); err != nil {
					errCh <- err
				}
			}(i)
		}

		wg.Wait()
		close(errCh)
		for err := range errCh {
			t.Errorf("create entry: %v", err)
		}
		if t.Failed() {
			t.FailNow()
		}

		got, err := store.List()
		if err != nil {
			t.Fatalf("list entries: %v", err)
		}
		if len(got) != count {
			t.Fatalf("got %d entries, want %d", len(got), count)
		}

		for i := range count {
			want := Entry{
				Key:   fmt.Sprintf("entry-%03d", i),
				Value: fmt.Sprintf("value-%d", i),
			}
			if got[i] != want {
				t.Errorf("entry %d: got %+v, want %+v", i, got[i], want)
			}
		}
	})
}
