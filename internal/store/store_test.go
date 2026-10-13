package store

import (
	"errors"
	"fmt"
	"sync"
	"testing"
)

func TestCollection(t *testing.T) {
	t.Run("retrieve and modify typed collection", func(t *testing.T) {
		type record struct {
			Name string
			Used bool
		}
		collection := newTestCollection[record](t)
		want := record{Name: "hello"}
		if err := collection.Create("key", want); err != nil {
			t.Fatalf("create entry: %v", err)
		}
		got, err := collection.Get("key")
		if err != nil {
			t.Fatalf("get entry: %v", err)
		}
		if got != want {
			t.Fatalf("got %+v, want %+v", got, want)
		}

		if err := collection.Modify("key", func(value *record) error {
			value.Used = true
			return nil
		}); err != nil {
			t.Fatalf("modify entry: %v", err)
		}
		got, err = collection.Get("key")
		if err != nil {
			t.Fatalf("get modified entry: %v", err)
		}
		want.Used = true
		if got != want {
			t.Errorf("got %+v, want %+v", got, want)
		}
	})

	t.Run("collections keep entries separate", func(t *testing.T) {
		db := newTestStore(t)
		first, err := NewCollection[string](db, "first")
		if err != nil {
			t.Fatalf("create first collection: %v", err)
		}
		second, err := NewCollection[string](db, "second")
		if err != nil {
			t.Fatalf("create second collection: %v", err)
		}
		if err := first.Create("hello", "one"); err != nil {
			t.Fatalf("create first entry: %v", err)
		}
		if err := second.Create("hello", "two"); err != nil {
			t.Fatalf("create second entry: %v", err)
		}

		if err := first.Delete("hello"); err != nil {
			t.Fatalf("delete first entry: %v", err)
		}
		got, err := second.Get("hello")
		if err != nil {
			t.Fatalf("get second entry: %v", err)
		}
		if got != "two" {
			t.Errorf("got %q, want %q", got, "two")
		}
	})

	t.Run("created entry can be retrieved", func(t *testing.T) {
		collection := newTestCollection[string](t)
		want := "world"

		if err := collection.Create("hello", want); err != nil {
			t.Fatalf("create entry: %v", err)
		}

		got, err := collection.Get("hello")
		if err != nil {
			t.Fatalf("get entry: %v", err)
		}
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("getting missing entry returns not found", func(t *testing.T) {
		collection := newTestCollection[string](t)

		_, err := collection.Get("missing")
		if !errors.Is(err, ErrNotFound) {
			t.Errorf("got error %v, want %v", err, ErrNotFound)
		}
	})

	t.Run("duplicate entry is rejected", func(t *testing.T) {
		collection := newTestCollection[string](t)
		original := "world"

		if err := collection.Create("hello", original); err != nil {
			t.Fatalf("create entry: %v", err)
		}
		if err := collection.Create("hello", "replacement"); !errors.Is(err, ErrAlreadyExists) {
			t.Fatalf("got error %v, want %v", err, ErrAlreadyExists)
		}

		got, err := collection.Get("hello")
		if err != nil {
			t.Fatalf("get entry: %v", err)
		}
		if got != original {
			t.Errorf("got %q, want unchanged %q", got, original)
		}
	})

	t.Run("entry can be modified", func(t *testing.T) {
		collection := newTestCollection[string](t)

		if err := collection.Create("hello", "world"); err != nil {
			t.Fatalf("create entry: %v", err)
		}

		err := collection.Modify("hello", func(value *string) error {
			*value += "!"
			return nil
		})
		if err != nil {
			t.Fatalf("modify entry: %v", err)
		}

		got, err := collection.Get("hello")
		if err != nil {
			t.Fatalf("get entry: %v", err)
		}
		if want := "world!"; got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("modifying missing entry returns not found", func(t *testing.T) {
		collection := newTestCollection[string](t)
		called := false

		err := collection.Modify("missing", func(value *string) error {
			called = true
			*value = "updated"
			return nil
		})
		if !errors.Is(err, ErrNotFound) {
			t.Fatalf("got error %v, want %v", err, ErrNotFound)
		}
		if called {
			t.Error("callback was called for a missing entry")
		}

		_, err = collection.Get("missing")
		if !errors.Is(err, ErrNotFound) {
			t.Errorf("got error %v after failed modify, want %v", err, ErrNotFound)
		}
	})

	t.Run("failed modification is not stored", func(t *testing.T) {
		collection := newTestCollection[string](t)
		original := "world"

		if err := collection.Create("hello", original); err != nil {
			t.Fatalf("create entry: %v", err)
		}

		wantErr := errors.New("callback failed")
		err := collection.Modify("hello", func(value *string) error {
			*value = "replacement"
			return wantErr
		})
		if !errors.Is(err, wantErr) {
			t.Fatalf("got error %v, want %v", err, wantErr)
		}

		got, err := collection.Get("hello")
		if err != nil {
			t.Fatalf("get entry: %v", err)
		}
		if got != original {
			t.Errorf("got %q, want unchanged %q", got, original)
		}
	})

	t.Run("entry can be deleted", func(t *testing.T) {
		collection := newTestCollection[string](t)

		if err := collection.Create("hello", "world"); err != nil {
			t.Fatalf("create entry: %v", err)
		}
		if err := collection.Delete("hello"); err != nil {
			t.Fatalf("delete entry: %v", err)
		}

		_, err := collection.Get("hello")
		if !errors.Is(err, ErrNotFound) {
			t.Errorf("got error %v, want %v", err, ErrNotFound)
		}
	})

	t.Run("deleting missing entry succeeds", func(t *testing.T) {
		collection := newTestCollection[string](t)

		if err := collection.Delete("missing"); err != nil {
			t.Fatalf("delete missing entry: %v", err)
		}
	})

	t.Run("entries can be listed in key order", func(t *testing.T) {
		collection := newTestCollection[string](t)

		for _, entry := range []Entry[string]{
			{Key: "charlie", Value: "three"},
			{Key: "alpha", Value: "one"},
			{Key: "bravo", Value: "two"},
		} {
			if err := collection.Create(entry.Key, entry.Value); err != nil {
				t.Fatalf("create entry: %v", err)
			}
		}

		got, err := collection.List()
		if err != nil {
			t.Fatalf("list entries: %v", err)
		}

		want := []Entry[string]{
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

	t.Run("empty collection lists no entries", func(t *testing.T) {
		collection := newTestCollection[string](t)

		got, err := collection.List()
		if err != nil {
			t.Fatalf("list entries: %v", err)
		}
		if len(got) != 0 {
			t.Errorf("got %d entries, want 0", len(got))
		}
	})

	t.Run("entries can be created concurrently", func(t *testing.T) {
		collection := newTestCollection[string](t)
		const count = 100

		errCh := make(chan error, count)
		var wg sync.WaitGroup

		for i := range count {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()

				key := fmt.Sprintf("entry-%03d", i)
				value := fmt.Sprintf("value-%d", i)
				if err := collection.Create(key, value); err != nil {
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

		got, err := collection.List()
		if err != nil {
			t.Fatalf("list entries: %v", err)
		}
		if len(got) != count {
			t.Fatalf("got %d entries, want %d", len(got), count)
		}

		for i := range count {
			want := Entry[string]{
				Key:   fmt.Sprintf("entry-%03d", i),
				Value: fmt.Sprintf("value-%d", i),
			}
			if got[i] != want {
				t.Errorf("entry %d: got %+v, want %+v", i, got[i], want)
			}
		}
	})
}
