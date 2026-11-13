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
