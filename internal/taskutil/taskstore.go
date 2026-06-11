package taskutil

import (
	"encoding/json"
	"slices"
	"sync"

	bolt "go.etcd.io/bbolt"
)

const taskBucket = "tasks"

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
		_, err := tx.CreateBucketIfNotExists([]byte(taskBucket))
		return err
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
func (ts *TaskStore) Put(rec TaskRecord) error {
	data, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	if err := ts.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(taskBucket))
		return b.Put([]byte(rec.Name), data)
	}); err != nil {
		return err
	}

	ts.notify(rec.Name)
	return nil
}
