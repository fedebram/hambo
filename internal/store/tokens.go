package store

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// Token lets one new node join, once, until it expires.
type Token struct {
	ID        []byte
	Secret    []byte
	ExpiresAt time.Time
}

// AddToken saves t.
func (s *Store) AddToken(ctx context.Context, t Token) error {
	_, err := s.write.ExecContext(ctx,
		"INSERT INTO tokens (id, secret, expires_at) VALUES (?, ?, ?)",
		t.ID, t.Secret, t.ExpiresAt.Unix())
	return err
}

// GetToken returns the token with this id. found is false if there is none.
func (s *Store) GetToken(ctx context.Context, id []byte) (t Token, found bool, err error) {
	var expires int64
	err = s.read.QueryRowContext(ctx,
		"SELECT id, secret, expires_at FROM tokens WHERE id = ?", id).Scan(&t.ID, &t.Secret, &expires)
	if errors.Is(err, sql.ErrNoRows) {
		return Token{}, false, nil
	}
	if err != nil {
		return Token{}, false, err
	}
	t.ExpiresAt = time.Unix(expires, 0)
	return t, true, nil
}

// DeleteExpiredTokens deletes the tokens that expired at now.
func (s *Store) DeleteExpiredTokens(ctx context.Context, now time.Time) error {
	_, err := s.write.ExecContext(ctx, "DELETE FROM tokens WHERE expires_at <= ?", now.Unix())
	return err
}

// UseToken deletes the token and adds the node.
// used is false if the token was already gone. It returns ErrConflict
// if the node's key, name or subnet is taken: then the token stays.
func (s *Store) UseToken(ctx context.Context, id []byte, n Node) (used bool, err error) {
	tx, err := s.write.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()

	res, err := tx.ExecContext(ctx, "DELETE FROM tokens WHERE id = ?", id)
	if err != nil {
		return false, err
	}
	deleted, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	if deleted == 0 {
		return false, nil
	}

	_, err = tx.ExecContext(ctx,
		"INSERT INTO nodes (public_key, name, created_at, wg_public, subnet) VALUES (?, ?, ?, ?, ?)",
		n.PublicKey, n.Name, n.CreatedAt.Unix(), n.WGPublic, n.Subnet)
	if isConflict(err) {
		return false, ErrConflict
	}
	if err != nil {
		return false, err
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}
	return true, nil
}
