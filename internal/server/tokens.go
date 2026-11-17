package server

import (
	"context"
	"crypto/rand"
	"time"

	"github.com/fedebram/hambo/internal/jointoken"
	"github.com/fedebram/hambo/internal/store"
)

// how long a join token stays valid
const tokenTTL = 24 * time.Hour

// createToken makes a new join token, and deletes the expired ones.
func (s *Server) createToken(ctx context.Context) (string, error) {
	now := time.Now()
	if err := s.st.DeleteExpiredTokens(ctx, now); err != nil {
		return "", err
	}

	token := store.Token{
		ID:        make([]byte, 6),
		Secret:    make([]byte, 32),
		ExpiresAt: now.Add(tokenTTL),
	}
	rand.Read(token.ID)
	rand.Read(token.Secret)
	if err := s.st.AddToken(ctx, token); err != nil {
		return "", err
	}
	return jointoken.Encode(s.key.Public, token.ID, token.Secret), nil
}
