package pki

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"errors"
	"fmt"
)

func GenerateNodeCSR(privateKey ed25519.PrivateKey) ([]byte, error) {
	if len(privateKey) != ed25519.PrivateKeySize {
		return nil, errors.New("invalid Ed25519 private key size")
	}
	der, err := x509.CreateCertificateRequest(
		rand.Reader,
		// leave the subject empty. the CN and O will be filled server side.
		&x509.CertificateRequest{},
		privateKey,
	)
	if err != nil {
		return nil, fmt.Errorf("create CSR: %w", err)
	}

	return der, nil
}
