package noiseconn

import (
	"crypto/rand"

	"github.com/flynn/noise"
)

// GenerateKey creates a new identity: a private and public key.
func GenerateKey() (noise.DHKey, error) {
	return noise.DH25519.GenerateKeypair(rand.Reader)
}
