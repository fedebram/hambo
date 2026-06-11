package taskutil

import (
	"path/filepath"
	"testing"
)

func newTestStore(t *testing.T) *TaskStore {
	t.Helper()
	ts, err := NewTaskStore(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("NewTaskStore: %v", err)
	}
	t.Cleanup(func() { ts.Close() })
	return ts
}

func rec(name string) TaskRecord {
	return TaskRecord{
		Name:  name,
		Image: "redis:alpine",
	}
}

func subsLen(ts *TaskStore) int {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	return len(ts.subs)
}

func assertChClosed(t *testing.T, ch <-chan struct{}) {
	t.Helper()
	select {
	case <-ch:
	default:
		t.Fatal("channel: want closed, got open")
	}
}

func assertChOpen(t *testing.T, ch <-chan struct{}) {
	t.Helper()
	select {
	case <-ch:
		t.Fatal("channel: want open, got closed")
	default:
	}
}

func TestGetMissing(t *testing.T) {
	ts := newTestStore(t)

	_, found, err := ts.Get("x")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if found {
		t.Fatal("found must be false")
	}
}

func TestPutGetRoundTrip(t *testing.T) {
	ts := newTestStore(t)

	v1 := rec("redis")
	if err := ts.Put(v1); err != nil {
		t.Fatalf("Put: %v", err)
	}

	got, found, err := ts.Get("redis")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !found {
		t.Fatal("found must be true")
	}
	if got != v1 {
		t.Errorf("got %+v, want %+v", got, v1)
	}

	v2 := rec("redis")
	v2.Image = "redis:latest"
	if err := ts.Put(v2); err != nil {
		t.Fatalf("Put: %v", err)
	}

	got, _, err = ts.Get("redis")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got != v2 {
		t.Errorf("after overwrite got %+v, want %+v", got, v2)
	}
}

// Multiple subscribers to the same key get notified at once.
func TestSubNotify(t *testing.T) {
	ts := newTestStore(t)

	a, _ := ts.Sub("redis")
	b, _ := ts.Sub("redis")
	other, cancelOther := ts.Sub("other")

	if err := ts.Put(rec("redis")); err != nil {
		t.Fatalf("Put: %v", err)
	}

	assertChClosed(t, a)
	assertChClosed(t, b)
	assertChOpen(t, other)

	if n := subsLen(ts); n != 1 {
		t.Fatalf("after notify: want 1 watched key (other), got %d", n)
	}
	cancelOther()
	if n := subsLen(ts); n != 0 {
		t.Fatalf("after cancel: want 0 watched keys, got %d", n)
	}
}

// After a subscription is notified the channel is closed, so you need to resubscribe.
func TestSubFiresOnce(t *testing.T) {
	ts := newTestStore(t)

	ch1, _ := ts.Sub("redis")
	if err := ts.Put(rec("redis")); err != nil {
		t.Fatalf("Put: %v", err)
	}
	assertChClosed(t, ch1)

	ch2, _ := ts.Sub("redis")
	assertChOpen(t, ch2)
	if err := ts.Put(rec("redis")); err != nil {
		t.Fatalf("Put: %v", err)
	}
	assertChClosed(t, ch2)
}
