package main

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
)

type tokenStore struct {
	mu     sync.Mutex
	tokens map[string][]byte
	path   string
}

type tokensFile struct {
	Tokens []tokenEntry `json:"tokens"`
}

type tokenEntry struct {
	ID     []byte `json:"id"`
	Secret []byte `json:"secret"`
}

func loadTokens(dataDir string) (*tokenStore, error) {
	ts := &tokenStore{
		tokens: make(map[string][]byte),
		path:   filepath.Join(dataDir, "tokens.json"),
	}

	data, err := os.ReadFile(ts.path)
	if errors.Is(err, fs.ErrNotExist) {
		return ts, nil // no tokens created yet
	}
	if err != nil {
		return nil, err
	}

	var f tokensFile
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("%s: %w", ts.path, err)
	}
	for _, t := range f.Tokens {
		ts.tokens[string(t.ID)] = t.Secret
	}
	return ts, nil
}

// create makes a new token.
// It returns the token's ID and secret.
func (ts *tokenStore) create() (id, secret []byte, err error) {
	id = make([]byte, 6)
	if _, err := rand.Read(id); err != nil {
		return nil, nil, err
	}
	secret = make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		return nil, nil, err
	}

	ts.mu.Lock()
	defer ts.mu.Unlock()

	ts.tokens[string(id)] = secret
	if err := ts.save(); err != nil {
		delete(ts.tokens, string(id))
		return nil, nil, err
	}
	return id, secret, nil
}

// save writes all tokens to the file. The caller must hold ts.mu.
func (ts *tokenStore) save() error {
	var f tokensFile
	for id, secret := range ts.tokens {
		f.Tokens = append(f.Tokens, tokenEntry{ID: []byte(id), Secret: secret})
	}
	data, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(ts.path, data, 0o600)
}

// joinToken puts the server's public key (32 bytes), the token ID (6 bytes)
// and the secret (32 bytes) one after the other, and encodes them as
// URL-safe base64 without padding.
func joinToken(serverPublic, id, secret []byte) string {
	var b []byte
	b = append(b, serverPublic...)
	b = append(b, id...)
	b = append(b, secret...)
	return base64.RawURLEncoding.EncodeToString(b)
}

// parseJoinToken reverses joinToken: it decodes s and splits it into
// the server's public key (32 bytes), the token ID (6) and the secret (32).
func parseJoinToken(s string) (serverPublic, id, secret []byte, err error) {
	b, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return nil, nil, nil, errors.New("invalid join token")
	}
	if len(b) != 32+6+32 {
		return nil, nil, nil, errors.New("invalid join token")
	}
	return b[0:32], b[32:38], b[38:70], nil
}
