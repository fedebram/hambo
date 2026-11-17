package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"

	"github.com/fedebram/hambo/internal/api"
	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

var ErrNameTaken = errors.New("name already taken")

func newID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// AddContainer saves a new container with this spec, giving it a new id.
// It returns ErrNameTaken if a container with the same name exists.
func (s *Store) AddContainer(ctx context.Context, spec api.ContainerSpec) (api.Container, error) {
	c := api.Container{
		ID:        newID(),
		Spec:      spec,
		CreatedAt: time.Now(),
	}

	data, err := json.Marshal(c.Spec)
	if err != nil {
		return api.Container{}, err
	}

	_, err = s.write.ExecContext(ctx,
		"INSERT INTO containers (id, name, spec, created_at) VALUES (?, ?, ?, ?)",
		c.ID, c.Spec.Name, data, c.CreatedAt.Unix())

	var sqliteErr *sqlite.Error
	if errors.As(err, &sqliteErr) && sqliteErr.Code() == sqlite3.SQLITE_CONSTRAINT_UNIQUE {
		return api.Container{}, ErrNameTaken
	}
	if err != nil {
		return api.Container{}, err
	}
	return c, nil
}

// ListContainers returns the containers, sorted by name.
func (s *Store) ListContainers(ctx context.Context) ([]api.Container, error) {
	// LEFT JOIN: the pending containers have no node, but we still want them
	rows, err := s.read.QueryContext(ctx,
		`SELECT ct.id, ct.spec, ct.created_at, cl.name
		FROM containers ct
		LEFT JOIN clients cl ON cl.public_key = ct.node_key
		ORDER BY ct.name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var containers []api.Container
	for rows.Next() {
		var c api.Container
		var data []byte
		var created int64
		// NULL while the container is pending
		var nodeName sql.NullString
		if err := rows.Scan(&c.ID, &data, &created, &nodeName); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(data, &c.Spec); err != nil {
			return nil, err
		}
		c.CreatedAt = time.Unix(created, 0)
		c.NodeName = nodeName.String
		containers = append(containers, c)
	}
	return containers, rows.Err()
}

// RemoveContainer deletes the container with this name.
// found is false if there is no such container.
func (s *Store) RemoveContainer(ctx context.Context, name string) (found bool, err error) {
	res, err := s.write.ExecContext(ctx, "DELETE FROM containers WHERE name = ?", name)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}
