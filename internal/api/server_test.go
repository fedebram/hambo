package api

import (
	"net/http"
	"testing"

	publicapi "github.com/fedebram/hambo/api"
)

// TDD approach inspired by https://quii.gitbook.io/learn-go-with-tests/build-an-application/http-server

func TestHealthEndpoint(t *testing.T) {
	t.Run("GET returns healthy status", func(t *testing.T) {
		srv := newTestServer(t)
		response := makeRequest(t, srv, http.MethodGet, "/health", nil)

		assertStatus(t, response.Code, http.StatusOK)

		assertContentType(t, response.Header(), "application/json")

		var got publicapi.HealthResponse

		decodeJSON(t, response.Body, &got)

		want := publicapi.HealthResponse{
			Status: "ok",
		}

		if got != want {
			t.Errorf("got %+v, want %+v", got, want)
		}
	})
}
