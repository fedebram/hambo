package container

import "time"

type fakeClock struct {
	current time.Time
	advance time.Duration
	calls   int
}

// newFakeClock creates a new clock to be used by the container service.
// The now function starts at start and advances in time based on the advance paramenter
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
