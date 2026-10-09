package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fedebram/hambo/api"
	internalapi "github.com/fedebram/hambo/internal/api"
)

func TestNewClient(t *testing.T) {
	httpClient := &http.Client{}
	endpoint := "http://127.0.0.1:8080"

	_, err := NewClient(endpoint, nil)
	if err == nil {
		t.Fatal("got nil error, want HTTP client validation error")
	}

	got, err := NewClient(endpoint, httpClient)
	if err != nil {
		t.Fatalf("unexpected NewClient error: %v", err)
	}

	if got.baseURL.String() != endpoint {
		t.Errorf("got base URL %q, want %q", got.baseURL.String(), endpoint)
	}

	if got.httpClient != httpClient {
		t.Error("HTTP client differs from the one passed to NewClient")
	}
}

func TestNewClientRejectsInvalidBaseURL(t *testing.T) {
	baseURLs := []string{
		"",
		"localhost:8080",
		"http://:8080",
		"http://localhost",
	}

	for _, baseURL := range baseURLs {
		_, err := NewClient(baseURL, &http.Client{})
		if err == nil {
			t.Errorf("NewClient with %q baseURL returned nil error", baseURL)
		}
	}
}

func TestHealth(t *testing.T) {
	server := httptest.NewServer(internalapi.NewServer(nil))
	t.Cleanup(server.Close)

	client, err := NewClient(server.URL, server.Client())
	if err != nil {
		t.Fatalf("unexpected NewClient error: %v", err)
	}

	got, err := client.Health(context.Background())
	if err != nil {
		t.Fatalf("unexpected Health error: %v", err)
	}

	want := api.HealthResponse{Status: "ok"}
	if got != want {
		t.Errorf("got response %+v, want %+v", got, want)
	}
}
