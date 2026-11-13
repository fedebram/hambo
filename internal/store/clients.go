package store

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// right now is fine too keep all the logic inside the store pkg!

// Role says what a client may do on the server.
type Role string

const (
	RoleUnknown Role = ""
	RoleAdmin   Role = "admin"
	RoleWorker  Role = "worker"
)

// Client is one known client.
type Client struct {
	PublicKey []byte
	Name      string
	Role      Role
	CreatedAt time.Time
}

// Role returns the role of the client with this public key,
// or RoleUnknown if there is none.
func (s *Store) Role(ctx context.Context, pub []byte) (Role, error) {
	var r Role
	err := s.read.QueryRowContext(ctx, "SELECT role FROM clients WHERE public_key = ?", pub).Scan(&r)
	if errors.Is(err, sql.ErrNoRows) {
		return RoleUnknown, nil
	}
	return r, err
}

// insertClient adds one client.
const insertClient = "INSERT INTO clients (public_key, name, role, created_at) VALUES (?, ?, ?, ?)"

// AddClient adds c. It fails if a client with the same public key exists.
func (s *Store) AddClient(ctx context.Context, c Client) error {
	_, err := s.write.ExecContext(ctx, insertClient, c.PublicKey, c.Name, c.Role, time.Now().Unix())
	return err
}

// ListNodes returns the workers, sorted by name.
func (s *Store) ListNodes(ctx context.Context) ([]Client, error) {
	rows, err := s.read.QueryContext(ctx,
		"SELECT public_key, name, role, created_at FROM clients WHERE role = ? ORDER BY name",
		RoleWorker)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var nodes []Client
	for rows.Next() {
		var c Client
		var created int64
		if err := rows.Scan(&c.PublicKey, &c.Name, &c.Role, &created); err != nil {
			return nil, err
		}
		c.CreatedAt = time.Unix(created, 0)
		nodes = append(nodes, c)
	}
	return nodes, rows.Err()
}
