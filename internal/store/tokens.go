package store

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"database/sql"
	"errors"
	"net/netip"
	"time"
)

// CreateToken makes a join token valid for ttl, and returns its id and secret.
// It also deletes the tokens that expired.
func (s *Store) CreateToken(ctx context.Context, ttl time.Duration) (id, secret []byte, err error) {
	id = make([]byte, 6)
	secret = make([]byte, 32)
	rand.Read(id) // never fails
	rand.Read(secret)

	now := time.Now()
	if _, err := s.write.ExecContext(ctx, "DELETE FROM tokens WHERE expires_at <= ?", now.Unix()); err != nil {
		return nil, nil, err
	}
	_, err = s.write.ExecContext(ctx,
		"INSERT INTO tokens (id, secret, expires_at) VALUES (?, ?, ?)",
		id, secret, now.Add(ttl).Unix(),
	)
	if err != nil {
		return nil, nil, err
	}
	return id, secret, nil
}

// Join uses up the token and adds the worker c, with a free /24 of cidr.
// It reports false if the token is unknown, expired or the secret is wrong,
// and returns ErrNoFreeSubnet if the cluster range is full.
func (s *Store) Join(ctx context.Context, id, secret []byte, c Client, cidr netip.Prefix) (bool, error) {
	tx, err := s.write.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()

	var stored []byte
	var expiresAt int64
	err = tx.QueryRowContext(ctx, "SELECT secret, expires_at FROM tokens WHERE id = ?", id).Scan(&stored, &expiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	// constant time
	if subtle.ConstantTimeCompare(stored, secret) != 1 {
		return false, nil
	}
	now := time.Now().Unix()
	if now >= expiresAt {
		return false, nil
	}

	if _, err := tx.ExecContext(ctx, "DELETE FROM tokens WHERE id = ?", id); err != nil {
		return false, err
	}

	taken, err := takenSubnets(ctx, tx)
	if err != nil {
		return false, err
	}
	subnet, err := freeSubnet(cidr, taken)
	if err != nil {
		return false, err
	}

	_, err = tx.ExecContext(ctx,
		"INSERT INTO clients (public_key, name, role, created_at, wg_public, subnet) VALUES (?, ?, ?, ?, ?, ?)",
		c.PublicKey, c.Name, c.Role, now, c.WGPublic, subnet.String())
	if err != nil {
		return false, err
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}
	return true, nil
}
