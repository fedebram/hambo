package taskutil

import (
	"encoding/json"
	"slices"
	"sync"

	bolt "go.etcd.io/bbolt"
)

const taskBucket = "tasks"
const sequenceBucket = "seq"

// TaskStore persists TaskRecords in a bbolt database and lets callers
// subscribe to changes on a given key.
type TaskStore struct {
	db *bolt.DB

	mu   sync.Mutex
	subs map[string][]chan struct{}
}

// NewTaskStore opens or creates the bbolt database at path and ensures the
// task bucket (tasks) exists.
func NewTaskStore(path string) (*TaskStore, error) {
	db, err := openDB(path)
	if err != nil {
		return nil, err
	}

	err = db.Update(func(tx *bolt.Tx) error {
		if _, err := tx.CreateBucketIfNotExists([]byte(taskBucket)); err != nil {
			return err
		}
		if _, err := tx.CreateBucketIfNotExists([]byte(sequenceBucket)); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		db.Close()
		return nil, err
	}
	return &TaskStore{
		db:   db,
		subs: make(map[string][]chan struct{}),
	}, nil
}

func openDB(path string) (*bolt.DB, error) {
	db, err := bolt.Open(path, 0600, nil)
	if err != nil {
		return nil, err
	}
	return db, nil
}

// Close closes the underlying database.
func (ts *TaskStore) Close() error {
	return ts.db.Close()
}

// Sub lets you subscribe to a key (name). The returned channel gets closed
// when the key is modified. The returned function serves to unsubscribe.
func (ts *TaskStore) Sub(name string) (<-chan struct{}, func()) {
	ch := make(chan struct{})

	ts.mu.Lock()
	ts.subs[name] = append(ts.subs[name], ch)
	ts.mu.Unlock()

	cancel := func() {
		ts.mu.Lock()
		defer ts.mu.Unlock()
		chs := ts.subs[name]
		if i := slices.Index(chs, ch); i >= 0 {
			ts.subs[name] = slices.Delete(chs, i, i+1)
		}
		if len(ts.subs[name]) == 0 {
			delete(ts.subs, name)
		}
	}
	return ch, cancel
}

func (ts *TaskStore) notify(name string) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	for _, ch := range ts.subs[name] {
		close(ch)
	}
	delete(ts.subs, name)
}

// Get returns the TaskRecord stored under name. The bool reports whether a
// record was found.
func (ts *TaskStore) Get(name string) (TaskRecord, bool, error) {
	var tr TaskRecord
	found := false
	err := ts.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(taskBucket))
		v := b.Get([]byte(name))
		if v == nil {
			return nil
		}
		found = true
		return json.Unmarshal(v, &tr)
	})
	if err != nil {
		return tr, found, err
	}

	return tr, found, nil
}

// Put stores rec under its Name, overwriting any existing record, and notifies
// every subscriber watching that key.
//
// Every successful write stamps the record with a global autoincrementing number
// (sequence number) that is also returned to the caller.
func (ts *TaskStore) Put(rec TaskRecord) (uint64, error) {
	var seq uint64
	if err := ts.db.Update(func(tx *bolt.Tx) error {
		var err error
		seq, err = tx.Bucket([]byte(sequenceBucket)).NextSequence()
		if err != nil {
			return err
		}
		rec.Seq = seq
		data, err := json.Marshal(rec)
		if err != nil {
			return err
		}
		b := tx.Bucket([]byte(taskBucket))
		return b.Put([]byte(rec.Name), data)
	}); err != nil {
		return 0, err
	}
	ts.notify(rec.Name)
	return seq, nil
}

// Update atomically reads the record under name and passes it to fn.
// fn returns the new record to be written and if it needs to be written.
// If fn returns true, then update writes the new record and returns the new sequence number.
// A Successful write notifies subscribers.
func (ts *TaskStore) Update(name string, fn func(cur TaskRecord, found bool) (TaskRecord, bool)) (uint64, error) {
	var (
		seq     uint64
		written bool
	)
	if err := ts.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(taskBucket))

		var cur TaskRecord
		found := false
		if v := b.Get([]byte(name)); v != nil {
			if err := json.Unmarshal(v, &cur); err != nil {
				return err
			}
			found = true
		}

		next, write := fn(cur, found)
		if !write {
			return nil
		}

		s, err := tx.Bucket([]byte(sequenceBucket)).NextSequence()
		if err != nil {
			return err
		}
		next.Seq = s
		data, err := json.Marshal(next)
		if err != nil {
			return err
		}
		if err := b.Put([]byte(name), data); err != nil {
			return err
		}
		seq, written = s, true
		return nil
	}); err != nil {
		return 0, err
	}

	if written {
		ts.notify(name)
	}
	return seq, nil
}

// TryDelete deletes a task record by name, but only if the stored sequence number
// still matches the seq the caller observed.
// Returns false if the seq doesn't match.
// Successful deletion notifies subscribers.
//
// If the record is already gone (already deleted by someone else), TryDelete returns true but does not notify subscribers.
func (ts *TaskStore) TryDelete(name string, seq uint64) (bool, error) {
	gone, removed := false, false
	if err := ts.db.Update(func(tx *bolt.Tx) error {
		var tr TaskRecord
		b := tx.Bucket([]byte(taskBucket))
		v := b.Get([]byte(name))
		if v == nil {
			gone = true
			return nil
		}
		if err := json.Unmarshal(v, &tr); err != nil {
			return err
		}
		if tr.Seq != seq {
			return nil
		}
		gone, removed = true, true
		return b.Delete([]byte(name))
	}); err != nil {
		return false, err
	}

	if removed {
		ts.notify(name)
	}
	return gone, nil
}
