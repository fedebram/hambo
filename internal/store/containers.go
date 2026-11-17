package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/fedebram/hambo/internal/api"
)

type Container struct {
	ID        string
	Spec      api.ContainerSpec
	NodeKey   []byte // the node it runs on, nil while pending
	CreatedAt time.Time
}

// AddContainer saves c, pending. It returns ErrConflict if the name is taken.
func (s *Store) AddContainer(ctx context.Context, c Container) error {
	spec, err := json.Marshal(c.Spec)
	if err != nil {
		return err
	}
	_, err = s.write.ExecContext(ctx,
		"INSERT INTO containers (id, name, spec, created_at) VALUES (?, ?, ?, ?)",
		c.ID, c.Spec.Name, spec, c.CreatedAt.Unix())
	if isConflict(err) {
		return ErrConflict
	}
	return err
}

// ListedContainer is a container with the name of its node.
type ListedContainer struct {
	Container
	NodeName string // empty while pending
}

// ListContainers returns the containers with the name of their node,
// sorted by container name.
func (s *Store) ListContainers(ctx context.Context) ([]ListedContainer, error) {
	// LEFT JOIN: the pending containers have no node, but we still want them
	rows, err := s.read.QueryContext(ctx,
		`SELECT ct.id, ct.spec, ct.node_key, ct.created_at, n.name
		FROM containers ct
		LEFT JOIN nodes n ON n.public_key = ct.node_key
		ORDER BY ct.name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var containers []ListedContainer
	for rows.Next() {
		var c ListedContainer
		var spec []byte
		var created int64
		var nodeName sql.NullString
		if err := rows.Scan(&c.ID, &spec, &c.NodeKey, &created, &nodeName); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(spec, &c.Spec); err != nil {
			return nil, err
		}
		c.CreatedAt = time.Unix(created, 0)
		c.NodeName = nodeName.String
		containers = append(containers, c)
	}
	return containers, rows.Err()
}

// ContainersAssignedTo returns the containers on the node, sorted by name.
func (s *Store) ContainersAssignedTo(ctx context.Context, node []byte) ([]Container, error) {
	rows, err := s.read.QueryContext(ctx,
		"SELECT id, spec, node_key, created_at FROM containers WHERE node_key = ? ORDER BY name", node)
	if err != nil {
		return nil, err
	}
	return readContainers(rows)
}

// readContainers reads rows of id, spec, node_key, created_at, and closes them.
func readContainers(rows *sql.Rows) ([]Container, error) {
	defer rows.Close()

	var containers []Container
	for rows.Next() {
		var c Container
		var spec []byte
		var created int64
		if err := rows.Scan(&c.ID, &spec, &c.NodeKey, &created); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(spec, &c.Spec); err != nil {
			return nil, err
		}
		c.CreatedAt = time.Unix(created, 0)
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

// NodeCount is a node and how many containers it has.
type NodeCount struct {
	PublicKey []byte
	Name      string
	Count     int
}

// ContainersPerNode returns every node with its number of containers,
// sorted by name.
func (s *Store) ContainersPerNode(ctx context.Context) ([]NodeCount, error) {
	// for each node, the subquery counts its containers (0 if none)
	rows, err := s.read.QueryContext(ctx,
		`SELECT public_key, name,
			(SELECT COUNT(*) FROM containers WHERE node_key = nodes.public_key)
		FROM nodes
		ORDER BY name`)
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
// It fails if the node doesn't exist (the foreign key).
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

// RemoveContainer deletes the container with this name and returns the
// key of its node, nil if it was pending. found is false if there is none.
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
