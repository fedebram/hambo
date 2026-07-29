package container

import (
	"testing"
	"testing/synctest"
	"time"
)

// Here we use the synctest package. It is useful to ensure deterministic behaviour when dealing with goroutines.
// Specifically we want to ensure that the goroutines are blocked before we make assertions.
// Without synctest we would need to use timers or who knows what else!
//
// synctest is useful also when dealing with time... maybe in the future we can remove the fake clock implementation for the service
//
// An article from the go blog that explains the synctest package:
// https://go.dev/blog/synctest

func TestQueueGetWaitsForWork(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		q := NewMemoryQueue()

		type result struct {
			name     string
			shutdown bool
		}

		got := make(chan result, 1)

		go func() {
			name, shutdown := q.Get()
			got <- result{name, shutdown}
		}()

		synctest.Wait()

		select {
		case r := <-got:
			t.Fatalf("get returned %+v while queue was empty", r)
		default:
		}
		want := result{name: "hello"}

		q.Add(want.name)

		synctest.Wait()

		select {
		case got := <-got:
			if got != want {
				t.Fatalf("got %+v, want %+v", got, want)
			}
		default:
			t.Fatal("get did not return after work was added")
		}

		if got := q.len(); got != 0 {
			t.Fatalf("queue length: %d, want 0", got)
		}
	})
}

func TestQueueShutdownWakesAllGetters(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		q := NewMemoryQueue()

		const getters = 4

		type result struct {
			name     string
			shutdown bool
		}

		got := make(chan result, getters)

		for range getters {
			go func() {
				name, shutdown := q.Get()
				got <- result{
					name:     name,
					shutdown: shutdown,
				}
			}()
		}

		synctest.Wait()

		select {
		case r := <-got:
			t.Fatalf("get returned before shutdown: %+v", r)
		default:
		}

		q.Shutdown()

		synctest.Wait()

		want := result{shutdown: true}
		for range getters {
			select {
			case result := <-got:
				if result != want {
					t.Errorf("got %+v, want %+v", result, want)
				}
			default:
				t.Fatal("shutdown did not wake every getter")
			}
		}
	})
}

func TestQueueGetFIFO(t *testing.T) {
	q := NewMemoryQueue()

	q.Add("first")
	q.Add("second")
	q.Add("third")

	if got := q.len(); got != 3 {
		t.Fatalf("queue length: %d, want 3", got)
	}

	for _, want := range []string{"first", "second", "third"} {
		got, shutdown := q.Get()

		if shutdown {
			t.Fatal("get returned shutdown true")
		}

		if got != want {
			t.Fatalf("got %q, want %q", got, want)
		}
	}

	if got := q.len(); got != 0 {
		t.Fatalf("queue length: %d, want 0", got)
	}
}

func TestQueueAddDoesNotDuplicateQueuedName(t *testing.T) {
	q := NewMemoryQueue()

	q.Add("hello")
	q.Add("hello")

	if got := q.len(); got != 1 {
		t.Fatalf("queue length: %d, want 1", got)
	}

	got, shutdown := q.Get()
	if shutdown {
		t.Fatal("get returned shutdown true")
	}
	if got != "hello" {
		t.Fatalf("got queued name %q, want %q", got, "hello")
	}

	if got := q.len(); got != 0 {
		t.Fatalf("queue length after get: %d, want 0", got)
	}
}

func TestQueueAddWhileProcessingRequeuesAfterDone(t *testing.T) {
	q := NewMemoryQueue()

	q.Add("hello")

	got, shutdown := q.Get()
	if shutdown {
		t.Fatal("get returned shutdown true")
	}
	if got != "hello" {
		t.Fatalf("got queued name %q, want %q", got, "hello")
	}

	q.Add("hello")

	if got := q.len(); got != 0 {
		t.Fatalf("queue length while name is being processed: %d, want 0", got)
	}

	q.Done("hello")

	if got := q.len(); got != 1 {
		t.Fatalf("queue length after done: %d, want 1", got)
	}

	got, shutdown = q.Get()
	if shutdown {
		t.Fatal("get returned shutdown true")
	}
	if got != "hello" {
		t.Fatalf("got requeued name %q, want %q", got, "hello")
	}
}

func TestQueueAddAfter(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		q := NewMemoryQueue()
		defer q.Shutdown()

		q.AddAfter("hello", time.Second)

		if got := q.len(); got != 0 {
			t.Fatalf("queue length before delay passed: %d, want 0", got)
		}

		// thanks synctest!!
		time.Sleep(time.Second)
		synctest.Wait()

		if got := q.len(); got != 1 {
			t.Fatalf("queue length after delay passed: %d, want 1", got)
		}

		got, shutdown := q.Get()
		if shutdown {
			t.Fatal("get returned shutdown true")
		}
		if got != "hello" {
			t.Fatalf("got queued name %q, want %q", got, "hello")
		}
	})
}

func TestQueueShutdownDiscardsDelayedWork(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		q := NewMemoryQueue()
		q.AddAfter("hello", time.Second)

		synctest.Wait()

		q.Shutdown()
		time.Sleep(time.Second)
		synctest.Wait()

		if got := q.len(); got != 0 {
			t.Fatalf("queue length after shutdown: %d, want 0", got)
		}
	})
}
