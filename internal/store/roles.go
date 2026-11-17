package store

import (
	"context"
	"database/sql"
	"errors"
)

// Role says what a key may do on the server.
type Role string

const (
	RoleUnknown Role = ""
	RoleAdmin   Role = "admin"
	RoleNode    Role = "node"
)

// Role returns the role of the pub key.
func (s *Store) Role(ctx context.Context, pub []byte) (Role, error) {
	var role Role
	err := s.read.QueryRowContext(ctx,
		`SELECT 'admin' FROM admins WHERE public_key = ?
		UNION ALL
		SELECT 'node' FROM nodes WHERE public_key = ?`, pub, pub).Scan(&role)
	if errors.Is(err, sql.ErrNoRows) {
		return RoleUnknown, nil
	}
	return role, err
}
