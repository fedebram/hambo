package api

import (
	"bytes"
	"encoding/json"
	"io"
	"mime"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TDD approach inspired by https://quii.gitbook.io/learn-go-with-tests/build-an-application/http-server

func TestHealthEndpoint(t *testing.T) {
	t.Run("GET returns healthy status", func(t *testing.T) {
		request := httptest.NewRequest(http.MethodGet, "/health", nil)
		response := httptest.NewRecorder()

		srv := NewServer()
		srv.ServeHTTP(response, request)

		assertStatus(t, response.Code, http.StatusOK)

		assertContentType(t, response.Header(), "application/json")

		var got healthResponse

		decodeJSON(t, response.Body, &got)

		want := healthResponse{
			Status: "ok",
		}

		if got != want {
			t.Errorf("got %+v, want %+v", got, want)
		}
	})
}

func TestContainersEndpoint(t *testing.T) {
	startTime := time.Date(2026, time.July, 19, 15, 0, 0, 0, time.UTC)
	clockCalls := 0

	// We need a fake clock so the created_at value in the response can be tested deterministically.
	// Not safe to be called concurrently.
	fakeClock := func() time.Time {
		current := startTime.Add(time.Duration(clockCalls) * time.Minute)
		clockCalls++
		return current
	}

	tests := []struct {
		name          string
		containerName string
		wantTime      time.Time
	}{
		{"creates hello container", "hello", startTime},
		{"creates database container", "database", startTime.Add(time.Minute)},
		{"creates worker container", "worker", startTime.Add(2 * time.Minute)},
	}

	srv := NewServer(WithClock(fakeClock))

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := jsonBody(t, createContainerRequest{Name: tt.containerName})
			request := httptest.NewRequest(http.MethodPost, "/containers", body)
			request.Header.Set("Content-Type", "application/json")
			response := httptest.NewRecorder()

			srv.ServeHTTP(response, request)

			assertStatus(t, response.Code, http.StatusCreated)
			assertContentType(t, response.Header(), "application/json")

			var got createContainerResponse
			decodeJSON(t, response.Body, &got)

			want := createContainerResponse{
				Name:      tt.containerName,
				CreatedAt: tt.wantTime,
			}

			if got != want {
				t.Errorf("got %+v, want %+v", got, want)
			}
		})
	}

	// Ensure the clock is called exactly once per request.
	if clockCalls != len(tests) {
		t.Errorf("clock called %d times, want %d", clockCalls, len(tests))
	}
}

// Most of this behaviour is tested in json_test.go. This test ensures that
// the handler uses srv.readJSON.
func TestCreateContainerRejectsUnknownJSONFields(t *testing.T) {
	body := strings.NewReader(`{"name":"hello","extra":true}`)
	request := httptest.NewRequest(http.MethodPost, "/containers", body)
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()

	NewServer().ServeHTTP(response, request)

	assertStatus(t, response.Code, http.StatusBadRequest)
}

func jsonBody(t *testing.T, data any) io.Reader {
	t.Helper()

	body, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("could not marshal JSON: %v", err)
	}

	return bytes.NewReader(body)
}

func assertStatus(t *testing.T, got, want int) {
	t.Helper()

	if got != want {
		t.Errorf(
			"got status %d (%s), want %d (%s)",
			got,
			http.StatusText(got),
			want,
			http.StatusText(want),
		)
	}
}

func assertContentType(t *testing.T, header http.Header, want string) {
	t.Helper()

	got, _, err := mime.ParseMediaType(header.Get("Content-Type"))
	if err != nil {
		t.Errorf("invalid content type: %v", err)
	}

	if got != want {
		t.Errorf("got content type %q, want %q", got, want)
	}
}

func decodeJSON(t *testing.T, r io.Reader, dst any) {
	t.Helper()

	if err := json.NewDecoder(r).Decode(dst); err != nil {
		t.Fatalf("could not decode JSON: %v", err)
	}
}
