package token

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"time"
)

// the shape of a token is :
// <CA fingerprint>.<secret>
// the ca fingerprint is the sha256 hash of the cert (DER) encoded as base64 safe url no padding format
// the secret is a randomly generated 32 bytes encoded as base64 safe url no padding format
// we use base64 encoding because it is compact.
//
// this takes inspiration from the token format of k3s see:
// https://docs.k3s.io/cli/token

// TODO: we need a way to delete used and expired tokens.

type Token struct {
	CAFingerprint string
	Secret        string
}

type Metadata struct {
	ExpiresAt  time.Time         `json:"expires_at"`
	Enrollment *EnrollmentResult `json:"enrollment,omitempty"`
}

type EnrollmentResult struct {
	RequestHash string `json:"request_hash"`
	NodeID      string `json:"node_id"`
	Certificate string `json:"certificate"`
}

func (m Metadata) Expired(now time.Time) bool {
	return !now.Before(m.ExpiresAt)
}

func (t Token) Format() string {
	return t.CAFingerprint + "." + t.Secret
}

func (t Token) SecretHash() string {
	digest := sha256.Sum256([]byte(t.Secret))
	return hex.EncodeToString(digest[:])
}

func Generate(caCert *x509.Certificate) (Token, error) {
	secret, err := generateSecret()
	if err != nil {
		return Token{}, err
	}

	return Token{
		CAFingerprint: caFingerprint(caCert),
		Secret:        secret,
	}, nil
}

func generateSecret() (string, error) {
	randomBytes := make([]byte, 32)
	// we handle the error, but in reality it will just panic in case of error
	if _, err := rand.Read(randomBytes); err != nil {
		return "", fmt.Errorf("generate token secret: %w", err)
	}

	return base64.RawURLEncoding.EncodeToString(randomBytes), nil
}

func caFingerprint(cert *x509.Certificate) string {
	digest := sha256.Sum256(cert.Raw)
	return base64.RawURLEncoding.EncodeToString(digest[:])
}
