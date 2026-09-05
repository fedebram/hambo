package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/fedebram/hambo/api"
)

type Client struct {
	baseURL    *url.URL
	httpClient *http.Client
}

func NewClient(baseURL string, httpClient *http.Client) (*Client, error) {
	if baseURL == "" {
		return nil, errors.New("base URL cannot be empty")
	}

	if httpClient == nil {
		return nil, errors.New("HTTP client cannot be nil")
	}

	u, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("parse base URL: %w", err)
	}

	if u.Scheme != "http" {
		return nil, fmt.Errorf("unsupported URL scheme %q", u.Scheme)
	}

	if u.Hostname() == "" {
		return nil, errors.New("base URL must include a hostname")
	}

	if u.Port() == "" {
		return nil, errors.New("base URL must include a port")
	}

	return &Client{
		baseURL:    u,
		httpClient: httpClient,
	}, nil
}

func (c *Client) Health(ctx context.Context) (api.HealthResponse, error) {
	var health api.HealthResponse
	err := c.do(ctx, http.MethodGet, "health", nil, &health)
	return health, err
}

func (c *Client) CreateContainer(ctx context.Context, input api.CreateContainerRequest) (api.Container, error) {
	var container api.Container
	err := c.do(ctx, http.MethodPost, "containers", input, &container)
	return container, err
}

func (c *Client) DeleteImage(ctx context.Context, name string) error {
	return c.do(ctx, http.MethodDelete, "images/"+url.PathEscape(name), nil, nil)
}

func (c *Client) do(ctx context.Context, method, path string, input, output any) error {
	var requestBody io.Reader
	if input != nil {
		body, err := json.Marshal(input)
		if err != nil {
			return fmt.Errorf("encode request: %w", err)
		}
		requestBody = bytes.NewReader(body)
	}

	endpoint := c.baseURL.JoinPath(path)
	req, err := http.NewRequestWithContext(ctx, method, endpoint.String(), requestBody)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	if input != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if output != nil {
		if err := json.NewDecoder(resp.Body).Decode(output); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
	}

	// I wonder if can be done better or in a different manner...
	if _, err := io.Copy(io.Discard, resp.Body); err != nil {
		return fmt.Errorf("finish reading response: %w", err)
	}

	return nil
}
