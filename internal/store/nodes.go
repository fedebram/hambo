package store

import (
	"context"
	"database/sql"
	"time"
)

// Node is a machine that joined the cluster.
type Node struct {
	PublicKey []byte
	Name      string
	CreatedAt time.Time
	WGPublic  []byte
	Subnet    string // like 10.100.1.0/24
	Endpoint  string // like 192.168.1.20:51820, empty until it first connects
}

// ListNodes returns the nodes, sorted by name.
func (s *Store) ListNodes(ctx context.Context) ([]Node, error) {
	rows, err := s.read.QueryContext(ctx,
		"SELECT public_key, name, created_at, wg_public, subnet, endpoint FROM nodes ORDER BY name")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var nodes []Node
	for rows.Next() {
		var n Node
		var created int64
		var endpoint sql.NullString
		if err := rows.Scan(&n.PublicKey, &n.Name, &created, &n.WGPublic, &n.Subnet, &endpoint); err != nil {
			return nil, err
		}
		n.CreatedAt = time.Unix(created, 0)
		n.Endpoint = endpoint.String
		nodes = append(nodes, n)
	}
	return nodes, rows.Err()
}

// ListSubnets returns the subnets the nodes have.
func (s *Store) ListSubnets(ctx context.Context) ([]string, error) {
	rows, err := s.read.QueryContext(ctx, "SELECT subnet FROM nodes")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var subnets []string
	for rows.Next() {
		var subnet string
		if err := rows.Scan(&subnet); err != nil {
			return nil, err
		}
		subnets = append(subnets, subnet)
	}
	return subnets, rows.Err()
}

// SetEndpoint saves the node's endpoint. changed is false if it was
// already this one.
func (s *Store) SetEndpoint(ctx context.Context, pub []byte, endpoint string) (changed bool, err error) {
	res, err := s.write.ExecContext(ctx,
		"UPDATE nodes SET endpoint = ? WHERE public_key = ? AND endpoint IS NOT ?",
		endpoint, pub, endpoint)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// RemoveNode deletes the node with this name, and its containers with it.
// found is false if there is no such node.
func (s *Store) RemoveNode(ctx context.Context, name string) (found bool, err error) {
	res, err := s.write.ExecContext(ctx, "DELETE FROM nodes WHERE name = ?", name)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}
