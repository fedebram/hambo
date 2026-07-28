package container

import (
	"testing"
	"testing/synctest"
)

// Here we use the synctest package. It is useful to ensure deterministic behaviour when dealing with goroutines.
// Specifically we want to ensure that the goroutines are blocked before we make assertions.
// Without synctest we would need to use timers or who knows what else!
//
// An article from the go blog that explains the synctest package:
// https://go.dev/blog/synctest

func TestQueueGetWaitsForWork(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		q := NewQueue()

		type result struct {
			name     string
			shutdown bool
		}

		got := make(chan result, 1)

		go func() {
			name, shutdown := q.get()
			got <- result{name, shutdown}
		}()

		synctest.Wait()

		select {
		case r := <-got:
			t.Fatalf("get returned %+v while queue was empty", r)
		default:
		}
		want := result{name: "hello"}

		q.add(want.name)

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
		q := NewQueue()

		const getters = 4

		type result struct {
			name     string
			shutdown bool
		}

		got := make(chan result, getters)

		for range getters {
			go func() {
				name, shutdown := q.get()
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

		q.shutdown()

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
	q := NewQueue()

	q.add("first")
	q.add("second")
	q.add("third")

	if got := q.len(); got != 3 {
		t.Fatalf("queue length: %d, want 3", got)
	}

	for _, want := range []string{"first", "second", "third"} {
		got, shutdown := q.get()

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
