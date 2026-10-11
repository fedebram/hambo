package pki

import (
	"crypto/ed25519"
	"crypto/x509"
)

type Certificate struct {
	Certificate *x509.Certificate
	PrivateKey  ed25519.PrivateKey
}

func (c *Certificate) Save(dir, name string) error {
	return saveCertificate(dir, name, c.Certificate, c.PrivateKey)
}
