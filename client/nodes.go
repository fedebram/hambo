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

func (c *Client) CreateNodeSession(ctx context.Context) (api.CreateNodeSessionResponse, error) {
	var response api.CreateNodeSessionResponse
	err := c.do(ctx, http.MethodPost, "nodes/self/session", nil, &response)
	return response, err
}

func (c *Client) ReportNodeStatus(ctx context.Context, request api.ReportNodeStatusRequest) error {
	return c.do(ctx, http.MethodPost, "nodes/self/status", request, nil)
}

// TODO: there are many repetitions here and also on fetch ca function. we need a way to unify everything.
// a self contained function with a client inside make sense for now because it is different from the mtls client.

func JoinNode(
	ctx context.Context,
	serverURL *url.URL,
	ca *x509.Certificate,
	secret string,
	request api.JoinNodeRequest,
) (api.JoinNodeResponse, error) {
	var zero api.JoinNodeResponse

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
		serverURL.JoinPath("nodes", "join").String(),
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

type NodeWatchReader struct {
	reader *JSONStreamReader
}

func (w *NodeWatchReader) Next() (api.NodeWatchResponse, error) {
	var response api.NodeWatchResponse
	err := w.reader.Decode(&response)
	return response, err
}

func (w *NodeWatchReader) Close() error {
	return w.reader.Close()
}

func (c *Client) WatchNode(ctx context.Context) (*NodeWatchReader, error) {
	reader, err := c.openStreamReader(ctx, http.MethodGet, "nodes/self/watch", nil)
	if err != nil {
		return nil, err
	}
	return &NodeWatchReader{reader: reader}, nil
}
