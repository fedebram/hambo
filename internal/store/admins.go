package store

import (
	"context"
	"time"
)

// Admin is a client that may manage the cluster, like the CLI.
type Admin struct {
	PublicKey []byte
	Name      string
	CreatedAt time.Time
}

// AddAdmin saves a. It returns ErrConflict if the key or the name exists.
func (s *Store) AddAdmin(ctx context.Context, a Admin) error {
	_, err := s.write.ExecContext(ctx,
		"INSERT INTO admins (public_key, name, created_at) VALUES (?, ?, ?)",
		a.PublicKey, a.Name, a.CreatedAt.Unix())
	if isConflict(err) {
		return ErrConflict
	}
	return err
}
