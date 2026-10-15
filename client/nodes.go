package client

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/fedebram/hambo/api"
)

// TODO: there are many repetitions here and also on fetch ca function. we need a way to unify everything.
// a self contained function with a client inside make sense for now because it is different from the mtls client.

func JoinNode(
	ctx context.Context,
	serverURL string,
	ca *x509.Certificate,
	secret string,
	request api.JoinNodeRequest,
) (api.JoinNodeResponse, error) {
	var zero api.JoinNodeResponse

	u, err := url.Parse(serverURL)
	if err != nil {
		return zero, fmt.Errorf("parse server URL: %w", err)
	}
	if u.Scheme != "https" || u.Hostname() == "" {
		return zero, errors.New("server URL must use HTTPS and include a hostname")
	}
	if u.User != nil {
		return zero, errors.New("server URL must not contain credentials")
	}
	if ca == nil {
		return zero, errors.New("CA certificate is required")
	}
	if secret == "" {
		return zero, errors.New("join token secret is required")
	}

	body, err := json.Marshal(request)
	if err != nil {
		return zero, fmt.Errorf("encode join request: %w", err)
	}

	roots := x509.NewCertPool()
	roots.AddCert(ca)

	// here we verify the server cert!
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{
		RootCAs: roots,
	}

	httpClient := &http.Client{
		Transport: transport,
		Timeout:   10 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	defer httpClient.CloseIdleConnections()

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		u.JoinPath("nodes", "join").String(),
		bytes.NewReader(body),
	)
	if err != nil {
		return zero, fmt.Errorf("create join request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+secret)

	resp, err := httpClient.Do(req)
	if err != nil {
		return zero, fmt.Errorf("send join request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return zero, readResponseError(ctx, resp)
	}

	const maxResponseSize = 64 * 1024

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize+1))
	if err != nil {
		return zero, fmt.Errorf("read join response: %w", err)
	}
	if len(data) > maxResponseSize {
		return zero, errors.New("join response is too large")
	}

	var response api.JoinNodeResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return zero, fmt.Errorf("decode join response: %w", err)
	}
	if response.NodeID == "" || response.Certificate == "" {
		return zero, errors.New("invalid join response: missing node ID or certificate")
	}

	return response, nil
}
