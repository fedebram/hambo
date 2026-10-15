package pki

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"errors"
	"fmt"
	"net"
	"path/filepath"
	"time"
)

type CA struct {
	Certificate *x509.Certificate
	PrivateKey  ed25519.PrivateKey
}

func GenerateCA(now time.Time) (*CA, error) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate CA key: %w", err)
	}

	template := &x509.Certificate{
		Subject: pkix.Name{
			CommonName: "Hambo CA",
		},
		NotBefore:             now.Add(-5 * time.Minute),
		NotAfter:              now.AddDate(5, 0, 0),
		BasicConstraintsValid: true,
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign,
	}
	der, err := x509.CreateCertificate(
		rand.Reader,
		template,
		template,
		publicKey,
		privateKey,
	)
	if err != nil {
		return nil, fmt.Errorf("create CA certificate: %w", err)
	}

	certificate, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, fmt.Errorf("parse CA certificate: %w", err)
	}

	return &CA{
		Certificate: certificate,
		PrivateKey:  privateKey,
	}, nil
}

func LoadCA(dir string) (*CA, error) {
	pair, err := tls.LoadX509KeyPair(
		filepath.Join(dir, "ca.crt"),
		filepath.Join(dir, "ca.key"),
	)
	if err != nil {
		return nil, fmt.Errorf("load CA certificate and key: %w", err)
	}

	certificate, err := x509.ParseCertificate(pair.Certificate[0])
	if err != nil {
		return nil, fmt.Errorf("parse CA certificate: %w", err)
	}

	if !certificate.BasicConstraintsValid || !certificate.IsCA {
		return nil, errors.New("CA certificate must have valid CA constraints")
	}
	if certificate.KeyUsage&x509.KeyUsageCertSign == 0 {
		return nil, errors.New("CA certificate must allow certificate signing")
	}

	privateKey, ok := pair.PrivateKey.(ed25519.PrivateKey)
	if !ok {
		return nil, errors.New("CA private key must use Ed25519")
	}

	return &CA{
		Certificate: certificate,
		PrivateKey:  privateKey,
	}, nil
}

func (ca *CA) Save(dir string) error {
	return saveCertificate(dir, "ca", ca.Certificate, ca.PrivateKey)
}

func (ca *CA) Remove(dir string) error {
	return removeCertificate(dir, "ca")
}

func (ca *CA) IssueServerCertificate(now time.Time) (*Certificate, error) {
	if now.Before(ca.Certificate.NotBefore) {
		return nil, errors.New("cannot issue server certificate: CA is not valid yet")
	}
	if !now.Before(ca.Certificate.NotAfter) {
		return nil, errors.New("cannot issue server certificate: CA has expired")
	}

	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate server key: %w", err)
	}

	template := &x509.Certificate{
		Subject: pkix.Name{
			CommonName: "Hambo Server",
		},
		NotBefore:             now.Add(-5 * time.Minute),
		NotAfter:              now.AddDate(0, 0, 90),
		BasicConstraintsValid: true,
		IsCA:                  false,
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:              []string{"localhost"},
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
	}

	if template.NotBefore.Before(ca.Certificate.NotBefore) {
		template.NotBefore = ca.Certificate.NotBefore
	}
	if template.NotAfter.After(ca.Certificate.NotAfter) {
		template.NotAfter = ca.Certificate.NotAfter
	}

	der, err := x509.CreateCertificate(
		rand.Reader,
		template,
		ca.Certificate,
		publicKey,
		ca.PrivateKey,
	)
	if err != nil {
		return nil, fmt.Errorf("create server certificate: %w", err)
	}

	certificate, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, fmt.Errorf("parse server certificate: %w", err)
	}

	return &Certificate{
		Certificate: certificate,
		PrivateKey:  privateKey,
	}, nil

}

func (ca *CA) IssueAdminCertificate(now time.Time) (*Certificate, error) {
	if now.Before(ca.Certificate.NotBefore) {
		return nil, errors.New("cannot issue admin certificate: CA is not valid yet")
	}
	if !now.Before(ca.Certificate.NotAfter) {
		return nil, errors.New("cannot issue admin certificate: CA has expired")
	}

	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate admin key: %w", err)
	}

	template := &x509.Certificate{
		Subject: pkix.Name{
			CommonName:   "admin",
			Organization: []string{"hambo:admins"},
		},
		NotBefore:             now.Add(-5 * time.Minute),
		NotAfter:              now.AddDate(0, 0, 90),
		BasicConstraintsValid: true,
		IsCA:                  false,
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}

	if template.NotBefore.Before(ca.Certificate.NotBefore) {
		template.NotBefore = ca.Certificate.NotBefore
	}
	if template.NotAfter.After(ca.Certificate.NotAfter) {
		template.NotAfter = ca.Certificate.NotAfter
	}

	der, err := x509.CreateCertificate(
		rand.Reader,
		template,
		ca.Certificate,
		publicKey,
		ca.PrivateKey,
	)
	if err != nil {
		return nil, fmt.Errorf("create admin certificate: %w", err)
	}

	certificate, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, fmt.Errorf("parse admin certificate: %w", err)
	}

	return &Certificate{
		Certificate: certificate,
		PrivateKey:  privateKey,
	}, nil
}

func (ca *CA) IssueNodeCertificate(now time.Time, nodeID string, csrDER []byte) (*x509.Certificate, error) {
	if nodeID == "" {
		return nil, errors.New("cannot issue node certificate: node ID is required")
	}
	if now.Before(ca.Certificate.NotBefore) {
		return nil, errors.New("cannot issue node certificate: CA is not valid yet")
	}
	if !now.Before(ca.Certificate.NotAfter) {
		return nil, errors.New("cannot issue node certificate: CA has expired")
	}

	csr, err := x509.ParseCertificateRequest(csrDER)
	if err != nil {
		return nil, fmt.Errorf("%w: parse node CSR: %w", ErrInvalidCSR, err)
	}

	publicKey, ok := csr.PublicKey.(ed25519.PublicKey)
	if !ok {
		return nil, fmt.Errorf("%w: node CSR must use an Ed25519 public key", ErrInvalidCSR)
	}

	if err := csr.CheckSignature(); err != nil {
		return nil, fmt.Errorf("%w: verify node CSR signature: %w", ErrInvalidCSR, err)
	}

	template := &x509.Certificate{
		Subject: pkix.Name{
			CommonName:   "hambo:node:" + nodeID,
			Organization: []string{"hambo:nodes"},
		},
		NotBefore:             now.Add(-5 * time.Minute),
		NotAfter:              now.AddDate(0, 0, 90),
		BasicConstraintsValid: true,
		IsCA:                  false,
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}

	if template.NotBefore.Before(ca.Certificate.NotBefore) {
		template.NotBefore = ca.Certificate.NotBefore
	}
	if template.NotAfter.After(ca.Certificate.NotAfter) {
		template.NotAfter = ca.Certificate.NotAfter
	}

	der, err := x509.CreateCertificate(
		rand.Reader,
		template,
		ca.Certificate,
		publicKey,
		ca.PrivateKey,
	)
	if err != nil {
		return nil, fmt.Errorf("create node certificate: %w", err)
	}

	certificate, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, fmt.Errorf("parse node certificate: %w", err)
	}

	return certificate, nil
}
