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

type Client struct {
	baseURL    *url.URL
	httpClient *http.Client
}

type JSONStreamReader struct {
	body    io.ReadCloser
	decoder *json.Decoder
}

func (r *JSONStreamReader) Decode(value any) error {
	// not safe to call concurrently!
	return r.decoder.Decode(value)
}

func (r *JSONStreamReader) Close() error {
	return r.body.Close()
}

type Config struct {
	ServerURL     string
	CACertPEM     []byte
	ClientCertPEM []byte
	ClientKeyPEM  []byte
}

func NewClient(cfg Config) (*Client, error) {
	u, err := ParseServerURL(cfg.ServerURL)
	if err != nil {
		return nil, fmt.Errorf("server URL: %w", err)
	}

	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(cfg.CACertPEM) {
		return nil, errors.New("CA PEM contains no parseable certificates")
	}
	certificate, err := tls.X509KeyPair(cfg.ClientCertPEM, cfg.ClientKeyPEM)
	if err != nil {
		return nil, fmt.Errorf("parse client certificate and key: %w", err)
	}

	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{
		RootCAs:      roots,
		Certificates: []tls.Certificate{certificate},
	}
	// limit waiting for headers without limiting how long a stream can stay open.
	transport.ResponseHeaderTimeout = 10 * time.Second
	httpClient := &http.Client{
		Transport: transport,
	}

	return &Client{
		baseURL:    u,
		httpClient: httpClient,
	}, nil
}

func (c *Client) CloseIdleConnections() {
	c.httpClient.CloseIdleConnections()
}

func (c *Client) Health(ctx context.Context) (api.HealthResponse, error) {
	var health api.HealthResponse
	err := c.do(ctx, http.MethodGet, "health", nil, &health)
	return health, err
}

func (c *Client) do(ctx context.Context, method, path string, input, output any) error {
	req, err := c.newRequest(ctx, method, path, input)
	if err != nil {
		return err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	// HTTP success in the range of 2XX
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return readResponseError(ctx, resp)
	}

	if output != nil {
		if err := json.NewDecoder(resp.Body).Decode(output); err != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return ctxErr
			}
			return fmt.Errorf("decode response: %w", err)
		}
	}

	return drainResponseBody(ctx, resp.Body)
}

func (c *Client) openStreamReader(ctx context.Context, method, path string, input any) (*JSONStreamReader, error) {
	req, err := c.newRequest(ctx, method, path, input)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/x-ndjson")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send streaming request: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		defer resp.Body.Close()
		return nil, readResponseError(ctx, resp)
	}

	// the caller owns the response body and must close the reader
	return &JSONStreamReader{
		body:    resp.Body,
		decoder: json.NewDecoder(resp.Body),
	}, nil
}

func (c *Client) newRequest(ctx context.Context, method string, path string, input any) (*http.Request, error) {
	var requestBody io.Reader
	if input != nil {
		body, err := json.Marshal(input)
		if err != nil {
			return nil, fmt.Errorf("encode request: %w", err)
		}
		requestBody = bytes.NewReader(body)
	}

	endpoint := c.baseURL.JoinPath(path)
	req, err := http.NewRequestWithContext(ctx, method, endpoint.String(), requestBody)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	if input != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	return req, nil
}

func drainResponseBody(ctx context.Context, body io.Reader) error {
	if _, err := io.Copy(io.Discard, body); err != nil {
		drainErr := fmt.Errorf("drain response body: %w", err)
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		return drainErr
	}
	return nil
}
