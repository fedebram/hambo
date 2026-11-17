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

// PendingContainers returns the ids of the containers without a node,
// the oldest first.
func (s *Store) PendingContainers(ctx context.Context) ([]string, error) {
	rows, err := s.read.QueryContext(ctx,
		"SELECT id FROM containers WHERE node_key IS NULL ORDER BY created_at")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// IsPending reports whether the container exists and has no node yet.
func (s *Store) IsPending(ctx context.Context, id string) (bool, error) {
	var pending bool
	err := s.read.QueryRowContext(ctx,
		"SELECT EXISTS (SELECT 1 FROM containers WHERE id = ? AND node_key IS NULL)", id).Scan(&pending)
	return pending, err
}

// NodeCount is a worker and how many containers it has.
type NodeCount struct {
	PublicKey []byte
	Name      string
	Count     int
}

// ContainersPerNode returns every node with its number of containers,
// sorted by name.
func (s *Store) ContainersPerNode(ctx context.Context) ([]NodeCount, error) {
	rows, err := s.read.QueryContext(ctx,
		`SELECT public_key, name,
			(SELECT COUNT(*) FROM containers WHERE node_key = clients.public_key)
		FROM clients
		WHERE role = ?
		ORDER BY name`, RoleWorker)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var counts []NodeCount
	for rows.Next() {
		var n NodeCount
		if err := rows.Scan(&n.PublicKey, &n.Name, &n.Count); err != nil {
			return nil, err
		}
		counts = append(counts, n)
	}
	return counts, rows.Err()
}

// AssignContainer puts the container on the node, only if it is still
// pending. assigned is false if it was removed or already has a node.
func (s *Store) AssignContainer(ctx context.Context, id string, node []byte) (assigned bool, err error) {
	res, err := s.write.ExecContext(ctx,
		"UPDATE containers SET node_key = ? WHERE id = ? AND node_key IS NULL",
		node, id)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// ContainersAssignedTo returns the containers assigned to the node,
// sorted by name.
func (s *Store) ContainersAssignedTo(ctx context.Context, node []byte) ([]api.NodeContainer, error) {
	rows, err := s.read.QueryContext(ctx,
		"SELECT id, spec FROM containers WHERE node_key = ? ORDER BY name", node)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var containers []api.NodeContainer
	for rows.Next() {
		var c api.NodeContainer
		var data []byte
		if err := rows.Scan(&c.ID, &data); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(data, &c.Spec); err != nil {
			return nil, err
		}
		containers = append(containers, c)
	}
	return containers, rows.Err()
}

// RemoveContainer deletes the container with this name and returns the
// key of its node, nil if it was pending.
// found is false if there is no such container.
func (s *Store) RemoveContainer(ctx context.Context, name string) (node []byte, found bool, err error) {
	err = s.write.QueryRowContext(ctx,
		"DELETE FROM containers WHERE name = ? RETURNING node_key", name).Scan(&node)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return node, true, nil
}
