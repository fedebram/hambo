package noiseconn

import (
	"crypto/rand"

	"github.com/flynn/noise"
)

var suite = noise.NewCipherSuite(noise.DH25519, noise.CipherChaChaPoly, noise.HashBLAKE2s)

// GenerateKey creates a new identity: a private and public key.
func GenerateKey() (noise.DHKey, error) {
	return suite.GenerateKeypair(rand.Reader)
}
