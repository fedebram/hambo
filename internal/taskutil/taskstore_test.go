package taskutil

import (
	"path/filepath"
	"reflect"
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
	seq1, err := ts.Put(v1)
	if err != nil {
		t.Fatalf("Put: %v", err)
	}

	got, found, err := ts.Get("redis")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !found {
		t.Fatal("found must be true")
	}
	if got.Seq != seq1 {
		t.Errorf("seq mismatch: got %d, want %d", got.Seq, seq1)
	}
	v1.Seq = got.Seq
	if !reflect.DeepEqual(got, v1) {
		t.Errorf("got %+v, want %+v", got, v1)
	}

	v2 := rec("redis")
	v2.Image = "redis:latest"
	seq2, err := ts.Put(v2)
	if err != nil {
		t.Fatalf("Put: %v", err)
	}

	got, found, err = ts.Get("redis")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !found {
		t.Fatal("found must be true")
	}
	if got.Seq != seq2 {
		t.Errorf("seq mismatch: got %d, want %d", got.Seq, seq2)
	}
	v2.Seq = got.Seq
	if !reflect.DeepEqual(got, v2) {
		t.Errorf("after overwrite got %+v, want %+v", got, v2)
	}
}

// Every put increments the sequence number
func TestSeqNumber(t *testing.T) {
	ts := newTestStore(t)
	r1 := rec("redis")
	_, err := ts.Put(r1)
	if err != nil {
		t.Fatalf("Put: %v", err)
	}
	r2 := rec("redis")
	_, err = ts.Put(r2)
	if err != nil {
		t.Fatalf("Put: %v", err)
	}

	// different key, the sequence keeps incrementing
	r3 := rec("nginx")
	seq3, err := ts.Put(r3)
	if err != nil {
		t.Fatalf("Put: %v", err)
	}

	if seq3 != 3 {
		t.Error("expected sequence 3 after three puts")
	}
}

// Multiple subscribers to the same key get notified at once.
func TestSubNotify(t *testing.T) {
	ts := newTestStore(t)

	a, _ := ts.Sub("redis")
	b, _ := ts.Sub("redis")
	other, cancelOther := ts.Sub("other")

	if _, err := ts.Put(rec("redis")); err != nil {
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
	if _, err := ts.Put(rec("redis")); err != nil {
		t.Fatalf("Put: %v", err)
	}
	assertChClosed(t, ch1)

	ch2, _ := ts.Sub("redis")
	assertChOpen(t, ch2)
	if _, err := ts.Put(rec("redis")); err != nil {
		t.Fatalf("Put: %v", err)
	}
	assertChClosed(t, ch2)
}

// Update on a missing key. fn sees found=false and creates the record.
func TestUpdateNotFound(t *testing.T) {
	ts := newTestStore(t)

	seq, err := ts.Update("redis", func(cur TaskRecord, found bool) (TaskRecord, bool) {
		if found {
			t.Error("found must be false: the record does not exist yet")
		}
		return rec("redis"), true
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if seq == 0 {
		t.Fatal("seq must be non-zero after a write")
	}

	got, found, err := ts.Get("redis")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !found {
		t.Fatal("record must exist after Update")
	}
	if got.Seq != seq {
		t.Errorf("seq mismatch: got %d, want %d", got.Seq, seq)
	}
}

// Update on an existing key. fn sees the current record and its changes persist.
func TestUpdateExist(t *testing.T) {
	ts := newTestStore(t)

	seq1, err := ts.Put(rec("redis"))
	if err != nil {
		t.Fatalf("Put: %v", err)
	}

	seq2, err := ts.Update("redis", func(cur TaskRecord, found bool) (TaskRecord, bool) {
		if !found {
			t.Error("found must be true: the record exists")
		}
		if cur.Seq != seq1 {
			t.Errorf("seq mismatch")
		}
		cur.Image = "redis:latest"
		return cur, true
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if seq2 <= seq1 {
		t.Errorf("seq must increase on write: got %d, want > %d", seq2, seq1)
	}

	got, _, err := ts.Get("redis")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Image != "redis:latest" {
		t.Errorf("image not updated: got %q", got.Image)
	}
}

// Update where fn returns write=false. nothing changes and nobody is notified.
func TestUpdateNoWrite(t *testing.T) {
	ts := newTestStore(t)

	seq1, err := ts.Put(rec("redis"))
	if err != nil {
		t.Fatalf("Put: %v", err)
	}

	ch, _ := ts.Sub("redis")

	seq, err := ts.Update("redis", func(cur TaskRecord, found bool) (TaskRecord, bool) {
		cur.Image = "should-not-be-saved"
		return cur, false
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if seq != 0 {
		t.Errorf("seq on update must be 0 when nothing is written, got %d", seq)
	}

	got, _, err := ts.Get("redis")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Seq != seq1 || got.Image != "redis:alpine" {
		t.Errorf("record must be unchanged, got %+v", got)
	}

	// no write means no notify!
	assertChOpen(t, ch)
}

// TryDelete with a matching seq removes the record and notifies subscribers.
func TestTryDeleteMatch(t *testing.T) {
	ts := newTestStore(t)

	seq, err := ts.Put(rec("redis"))
	if err != nil {
		t.Fatalf("Put: %v", err)
	}

	ch, _ := ts.Sub("redis")

	ok, err := ts.TryDelete("redis", seq)
	if err != nil {
		t.Fatalf("TryDelete: %v", err)
	}
	if !ok {
		t.Fatal("TryDelete must return true on a matching seq")
	}

	_, found, err := ts.Get("redis")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if found {
		t.Fatal("record must be gone after delete")
	}

	// we successfully deleted the record, so subscribers gets notified (ch closed)
	assertChClosed(t, ch)
}

// TryDelete with a stale seq deletes nothing and notifies nobody.
func TestTryDeleteStaleSeq(t *testing.T) {
	ts := newTestStore(t)

	seq1, err := ts.Put(rec("redis"))
	if err != nil {
		t.Fatalf("Put: %v", err)
	}
	// write a second time so seq increments
	if _, err := ts.Put(rec("redis")); err != nil {
		t.Fatalf("Put: %v", err)
	}

	ch, _ := ts.Sub("redis")

	// stale seq!
	ok, err := ts.TryDelete("redis", seq1)
	if err != nil {
		t.Fatalf("TryDelete: %v", err)
	}
	if ok {
		t.Fatal("TryDelete must return false on a stale seq")
	}

	_, found, err := ts.Get("redis")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !found {
		t.Fatal("record must still exist after a failed delete")
	}

	// failed delete, no notify, ch remains open
	assertChOpen(t, ch)
}

// TryDelete on a record that is already gone (already deleted by someone else) reports success but notifies nobody.
func TestTryDeleteAlreadyGone(t *testing.T) {
	ts := newTestStore(t)

	ch, _ := ts.Sub("x")

	ok, err := ts.TryDelete("x", 1)
	if err != nil {
		t.Fatalf("TryDelete: %v", err)
	}
	if !ok {
		t.Fatal("TryDelete must return true when the record is already gone")
	}

	// nothing to delete! no one to notify!
	assertChOpen(t, ch)
}
