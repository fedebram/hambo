package worker

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"os"

	"github.com/fedebram/hambo/api"
	"github.com/fedebram/hambo/client"
	"github.com/fedebram/hambo/internal/pki"
	"github.com/fedebram/hambo/internal/token"
)

type JoinConfig struct {
	ServerURL string
	Token     string
	Name      string
	DataDir   string
}

// the join is idempotent! as long as the token is not expired. If there is something wrong during the enrollment, the enrollment state is saved.
// recall join retrieves the saved state and retry to enroll.

func Join(ctx context.Context, cfg JoinConfig) error {
	joinToken, err := token.Parse(cfg.Token)
	if err != nil {
		return fmt.Errorf("parse join token: %w", err)
	}

	pending, err := prepareEnrollment(cfg)
	if err != nil {
		return err
	}

	ca, err := client.FetchAndVerifyCA(
		ctx,
		pending.ServerURL,
		joinToken.CAFingerprint,
	)
	if err != nil {
		return fmt.Errorf("fetch and verify CA: %w", err)
	}

	response, err := client.JoinNode(
		ctx,
		pending.ServerURL,
		ca,
		joinToken.Secret,
		api.JoinNodeRequest{
			Name: pending.Name,
			CSR:  pending.CSR,
		},
	)
	if err != nil {
		return fmt.Errorf("join node: %w", err)
	}

	if _, err := tls.X509KeyPair(
		[]byte(response.Certificate),
		[]byte(pending.PrivateKey),
	); err != nil {
		return fmt.Errorf("parse worker certificate and key: %w", err)
	}

	pending.NodeID = response.NodeID
	pending.Certificate = response.Certificate
	pending.CACertificate = string(pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: ca.Raw,
	}))

	// keep the private key; enrollment retry data is no longer needed.
	pending.Token = ""
	pending.CSR = nil

	if err := saveState(cfg.DataDir, pending); err != nil {
		return fmt.Errorf("save worker credentials: %w", err)
	}

	return nil

}

func prepareEnrollment(cfg JoinConfig) (*state, error) {
	saved, err := loadState(cfg.DataDir)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}

	if err == nil {
		if saved.NodeID != "" {
			return nil, errors.New("worker has already joined")
		}
		if saved.Token == cfg.Token {
			if saved.ServerURL != cfg.ServerURL || saved.Name != cfg.Name {
				return nil, errors.New(
					"server or name differs from the saved enrollment",
				)
			}
			if saved.PrivateKey == "" || len(saved.CSR) == 0 {
				return nil, errors.New("saved enrollment is incomplete")
			}

			return saved, nil
		}
	}

	// no saved enrollment attempt or different token -> create a new enrollment

	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate worker private key: %w", err)
	}

	csr, err := pki.GenerateNodeCSR(privateKey)
	if err != nil {
		return nil, err
	}

	privateKeyDER, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		return nil, fmt.Errorf("encode worker private key: %w", err)
	}

	pending := &state{
		ServerURL: cfg.ServerURL,
		Token:     cfg.Token,
		Name:      cfg.Name,
		PrivateKey: string(pem.EncodeToMemory(&pem.Block{
			Type:  "PRIVATE KEY",
			Bytes: privateKeyDER,
		})),
		CSR: csr,
	}

	if err := saveState(cfg.DataDir, pending); err != nil {
		return nil, err
	}

	return pending, nil
}
