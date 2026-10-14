package client

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

func FetchAndVerifyCA(ctx context.Context, serverURL string, expectedFingerprint string) (*x509.Certificate, error) {
	u, err := url.Parse(serverURL)
	if err != nil {
		return nil, fmt.Errorf("parse server URL: %w", err)
	}
	if u.Scheme != "https" || u.Hostname() == "" {
		return nil, errors.New("server URL must use HTTPS and include a hostname")
	}
	if u.User != nil {
		return nil, errors.New("server URL must not contain credentials")
	}

	if expectedFingerprint == "" {
		return nil, errors.New("CA fingerprint is required")
	}

	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{
		// server verification here is disabled because we need to fetch the CA!
		// We don't trust the downloaded CA until its fingerprint matches.
		InsecureSkipVerify: true,
	}

	httpClient := &http.Client{
		Transport: transport,
		Timeout:   10 * time.Second,
		// prevent redirect
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	defer httpClient.CloseIdleConnections()

	req, err := http.NewRequestWithContext(
		ctx, http.MethodGet, u.JoinPath("ca").String(), nil,
	)
	if err != nil {
		return nil, fmt.Errorf("create CA request: %w", err)
	}
	req.Header.Set("Accept", "application/pem-certificate-chain")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch CA certificate: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch CA certificate: status %d", resp.StatusCode)
	}

	const maxCASize = 64 * 1024
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxCASize+1))
	if err != nil {
		return nil, fmt.Errorf("read CA certificate: %w", err)
	}
	if len(data) > maxCASize {
		return nil, errors.New("CA response is too large")
	}

	block, _ := pem.Decode(data)
	if block == nil || block.Type != "CERTIFICATE" {
		return nil, errors.New("CA response must contain a PEM certificate")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse CA certificate: %w", err)
	}

	// i wonder if this can be "standardized" with an external function
	digest := sha256.Sum256(cert.Raw)
	fingerprint := base64.RawURLEncoding.EncodeToString(digest[:])
	if fingerprint != expectedFingerprint {
		return nil, errors.New("CA fingerprint does not match")
	}

	return cert, nil
}
