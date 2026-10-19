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
	"net/url"
	"os"
	"strings"

	"github.com/fedebram/hambo/api"
	"github.com/fedebram/hambo/client"
	"github.com/fedebram/hambo/internal/pki"
	"github.com/fedebram/hambo/internal/token"
)

type JoinConfig struct {
	ServerURL *url.URL
	Token     string
	Name      string
	DataDir   string
}

type JoinResult struct {
	NodeID            string
	AlreadyConfigured bool
}

func Join(ctx context.Context, cfg JoinConfig) (JoinResult, error) {
	lock, err := lockDataDir(cfg.DataDir)
	if err != nil {
		return JoinResult{}, err
	}
	defer lock.Close()

	saved, err := loadState(cfg.DataDir)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return JoinResult{}, err
	}
	if errors.Is(err, os.ErrNotExist) {
		if strings.TrimSpace(cfg.Token) == "" {
			return JoinResult{}, errors.New("join token is required for enrollment")
		}
		joinToken, err := token.Parse(cfg.Token)
		if err != nil {
			return JoinResult{}, fmt.Errorf("parse join token: %w", err)
		}

		pending, err := newEnrollment(cfg)
		if err != nil {
			return JoinResult{}, err
		}

		// save before contacting the server so an interrupted enrollment can be retried
		if err := saveState(cfg.DataDir, pending); err != nil {
			return JoinResult{}, err
		}

		return enroll(ctx, cfg, pending, joinToken)
	}

	if saved.ServerURL != cfg.ServerURL.String() || saved.Name != cfg.Name {
		return JoinResult{}, errors.New(
			"server or name differs from saved enrollment: run reset first",
		)
	}

	// resume the saved pending enrollment.
	if saved.NodeID == "" {
		// a pending enrollment shouldn't contain completed credentials
		if saved.Certificate != "" || saved.CACertificate != "" {
			return JoinResult{}, errors.New("saved enrollment state is inconsistent")
		}

		// check the fields needed to resume the original request
		if saved.Token == "" ||
			saved.PrivateKey == "" ||
			len(saved.CSR) == 0 {
			return JoinResult{}, errors.New("saved enrollment is incomplete")
		}
		if cfg.Token != "" && cfg.Token != saved.Token {
			return JoinResult{}, errors.New(
				"token differs from saved enrollment: run reset first",
			)
		}

		joinToken, err := token.Parse(saved.Token)
		if err != nil {
			return JoinResult{}, fmt.Errorf("parse join token: %w", err)
		}

		return enroll(ctx, cfg, saved, joinToken)
	}

	// completed enrollment: check that the required credentials are present
	if saved.PrivateKey == "" ||
		saved.Certificate == "" ||
		saved.CACertificate == "" {
		return JoinResult{}, errors.New("saved worker credentials are incomplete")
	}
	return JoinResult{NodeID: saved.NodeID, AlreadyConfigured: true}, nil
}

func newEnrollment(cfg JoinConfig) (*state, error) {
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
		ServerURL: cfg.ServerURL.String(),
		Token:     cfg.Token,
		Name:      cfg.Name,
		PrivateKey: string(pem.EncodeToMemory(&pem.Block{
			Type:  "PRIVATE KEY",
			Bytes: privateKeyDER,
		})),
		CSR: csr,
	}

	return pending, nil
}

func enroll(ctx context.Context, cfg JoinConfig, pending *state, joinToken token.Token) (JoinResult, error) {
	ca, err := client.FetchAndVerifyCA(
		ctx,
		cfg.ServerURL,
		joinToken.CAFingerprint,
	)
	if err != nil {
		return JoinResult{}, fmt.Errorf("fetch and verify CA: %w", err)
	}

	response, err := client.JoinNode(
		ctx,
		cfg.ServerURL,
		ca,
		joinToken.Secret,
		api.JoinNodeRequest{
			Name: pending.Name,
			CSR:  pending.CSR,
		},
	)
	if err != nil {
		return JoinResult{}, fmt.Errorf("join node: %w", err)
	}

	if _, err := tls.X509KeyPair(
		[]byte(response.Certificate),
		[]byte(pending.PrivateKey),
	); err != nil {
		return JoinResult{}, fmt.Errorf("parse worker certificate and key: %w", err)
	}

	pending.NodeID = response.NodeID
	pending.Certificate = response.Certificate
	pending.CACertificate = string(pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: ca.Raw,
	}))
	// completed credentials. No need for token and csr.
	pending.Token = ""
	pending.CSR = nil

	if err := saveState(cfg.DataDir, pending); err != nil {
		return JoinResult{}, fmt.Errorf("save worker credentials: %w", err)
	}

	return JoinResult{NodeID: pending.NodeID}, nil
}
