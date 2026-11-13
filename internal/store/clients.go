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

	// only for workers
	WGPublic []byte
	Subnet   string // like 10.100.1.0/24
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

// AddClient adds c, without wireguard key and subnet (used for the admin).
// It fails if a client with the same public key exists.
func (s *Store) AddClient(ctx context.Context, c Client) error {
	_, err := s.write.ExecContext(ctx,
		"INSERT INTO clients (public_key, name, role, created_at) VALUES (?, ?, ?, ?)",
		c.PublicKey, c.Name, c.Role, time.Now().Unix())
	return err
}

// ListNodes returns the workers, sorted by name.
func (s *Store) ListNodes(ctx context.Context) ([]Client, error) {
	rows, err := s.read.QueryContext(ctx,
		"SELECT public_key, name, role, created_at, wg_public, subnet FROM clients WHERE role = ? ORDER BY name",
		RoleWorker)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var nodes []Client
	for rows.Next() {
		var c Client
		var created int64
		if err := rows.Scan(&c.PublicKey, &c.Name, &c.Role, &created, &c.WGPublic, &c.Subnet); err != nil {
			return nil, err
		}
		c.CreatedAt = time.Unix(created, 0)
		nodes = append(nodes, c)
	}
	return nodes, rows.Err()
}

// RemoveNode deletes the worker with this name and returns its public key.
// found is false if there is no such worker.
func (s *Store) RemoveNode(ctx context.Context, name string) (pub []byte, found bool, err error) {
	err = s.write.QueryRowContext(ctx,
		"DELETE FROM clients WHERE name = ? AND role = ? RETURNING public_key",
		name, RoleWorker).Scan(&pub)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return pub, true, nil
}
