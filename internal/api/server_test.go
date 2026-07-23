package api

import (
	"errors"
	"log/slog"
	"maps"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/fedebram/hambo/internal/container"
)

// TDD approach inspired by https://quii.gitbook.io/learn-go-with-tests/build-an-application/http-server

func TestHealthEndpoint(t *testing.T) {
	t.Run("GET returns healthy status", func(t *testing.T) {
		srv := newTestServer(t)
		response := makeRequest(t, srv, http.MethodGet, "/health", nil)

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

func TestCreateContainer(t *testing.T) {
	startTime := time.Date(2026, time.July, 19, 15, 0, 0, 0, time.UTC)
	clock := newFakeClock(startTime, time.Minute)

	tests := []struct {
		name          string
		containerName string
		wantTime      time.Time
	}{
		{"creates hello container", "hello", startTime},
		{"creates database container", "database", startTime.Add(time.Minute)},
		{"creates worker container", "worker", startTime.Add(2 * time.Minute)},
	}

	srv := newTestServer(t, WithClock(clock.now))

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response := makeRequest(t, srv, http.MethodPost, "/containers", createContainerRequest{
				Name:  tt.containerName,
				Image: "docker.io/library/alpine:latest",
			})

			assertStatus(t, response.Code, http.StatusCreated)
			assertContentType(t, response.Header(), "application/json")

			var got container.Container
			decodeJSON(t, response.Body, &got)

			want := container.Container{
				Name:      tt.containerName,
				Image:     "docker.io/library/alpine:latest",
				CreatedAt: tt.wantTime,
			}

			if got != want {
				t.Errorf("got %+v, want %+v", got, want)
			}
		})
	}

	// Ensure the clock is called exactly once per request.
	if clock.calls != len(tests) {
		t.Errorf("clock called %d times, want %d", clock.calls, len(tests))
	}
}

// Most of this behaviour is tested in json_test.go. This test ensures that
// the handler uses srv.readJSON.
func TestCreateContainerRejectsUnknownJSONFields(t *testing.T) {
	body := strings.NewReader(`{"name":"hello","extra":true}`)
	request := httptest.NewRequest(http.MethodPost, "/containers", body)
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()

	newTestServer(t).ServeHTTP(response, request)

	assertStatus(t, response.Code, http.StatusBadRequest)
}

func TestCreateContainerRejectsMissingImage(t *testing.T) {
	srv := newTestServer(t)

	response := makeRequest(t, srv, http.MethodPost, "/containers", struct {
		Name string `json:"name"`
	}{
		Name: "hello",
	})

	requireStatus(t, response.Code, http.StatusBadRequest)
	assertContentType(t, response.Header(), "application/json")

	var got struct {
		Errors map[string]string `json:"errors"`
	}
	decodeJSON(t, response.Body, &got)

	if got.Errors["image"] != "must be provided" {
		t.Errorf("got image error %q, want %q", got.Errors["image"], "must be provided")
	}

	getResponse := makeRequest(t, srv, http.MethodGet, "/containers/hello", nil)
	assertStatus(t, getResponse.Code, http.StatusNotFound)
}

func TestCreateContainerReturnsAllValidationErrors(t *testing.T) {
	type validationResponse struct {
		Errors map[string]string `json:"errors"`
	}

	response := makeRequest(t, newTestServer(t), http.MethodPost, "/containers", struct{}{})

	requireStatus(t, response.Code, http.StatusUnprocessableEntity)
	assertContentType(t, response.Header(), "application/json")

	var got validationResponse
	decodeJSON(t, response.Body, &got)

	want := validationResponse{
		Errors: map[string]string{
			"name":  "must be provided",
			"image": "must be provided",
		},
	}

	if !maps.Equal(got.Errors, want.Errors) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestGetMissingContainerReturnsNotFound(t *testing.T) {
	response := makeRequest(t, newTestServer(t), http.MethodGet, "/containers/missing", nil)

	assertStatus(t, response.Code, http.StatusNotFound)
}

func TestContainerStoreFailuresReturnInternalServerError(t *testing.T) {
	tests := []struct {
		name   string
		method string
		path   string
		body   any
	}{
		{"GET", http.MethodGet, "/containers/hello", nil},
		{"POST", http.MethodPost, "/containers", createContainerRequest{
			Name:  "hello",
			Image: "docker.io/library/alpine:latest",
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := slog.New(slog.DiscardHandler)
			store := failingStore{err: errors.New("store unavailable")}
			response := makeRequest(t, NewServer(store, WithLogger(logger)), tt.method, tt.path, tt.body)

			assertStatus(t, response.Code, http.StatusInternalServerError)
		})
	}
}

func TestCreateAndGetContainer(t *testing.T) {
	fixedTime := time.Date(2026, time.July, 19, 15, 0, 0, 0, time.UTC)
	clock := newFakeClock(fixedTime, 0)
	srv := newTestServer(t, WithClock(clock.now))

	createResponse := makeRequest(t, srv, http.MethodPost, "/containers", createContainerRequest{
		Name:  "hello",
		Image: "docker.io/library/alpine:latest",
	})

	requireStatus(t, createResponse.Code, http.StatusCreated)

	getResponse := makeRequest(t, srv, http.MethodGet, "/containers/hello", nil)

	requireStatus(t, getResponse.Code, http.StatusOK)
	assertContentType(t, getResponse.Header(), "application/json")

	var got container.Container
	decodeJSON(t, getResponse.Body, &got)

	want := container.Container{
		Name:      "hello",
		Image:     "docker.io/library/alpine:latest",
		CreatedAt: fixedTime,
	}

	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestCreateContainerRejectsDuplicateName(t *testing.T) {
	firstTime := time.Date(2026, time.July, 19, 15, 0, 0, 0, time.UTC)
	clock := newFakeClock(firstTime, time.Minute)
	srv := newTestServer(t, WithClock(clock.now))

	firstResponse := makeRequest(t, srv, http.MethodPost, "/containers", createContainerRequest{
		Name:  "hello",
		Image: "docker.io/library/alpine:latest",
	})

	requireStatus(t, firstResponse.Code, http.StatusCreated)

	duplicateResponse := makeRequest(t, srv, http.MethodPost, "/containers", createContainerRequest{
		Name:  "hello",
		Image: "docker.io/library/alpine:latest",
	})

	assertStatus(t, duplicateResponse.Code, http.StatusConflict)

	getResponse := makeRequest(t, srv, http.MethodGet, "/containers/hello", nil)

	requireStatus(t, getResponse.Code, http.StatusOK)

	var got container.Container
	decodeJSON(t, getResponse.Body, &got)

	want := container.Container{
		Name:      "hello",
		Image:     "docker.io/library/alpine:latest",
		CreatedAt: firstTime,
	}

	if got != want {
		t.Errorf("got %+v, want original %+v", got, want)
	}
}

func TestGetReturnsContainerRunningState(t *testing.T) {
	fixedTime := time.Date(2026, time.July, 19, 15, 0, 0, 0, time.UTC)
	store := container.NewMemoryStore()
	srv := NewServer(store)

	if err := store.Create(container.Container{
		Name:      "hello",
		State:     container.StateRunning,
		CreatedAt: fixedTime,
	}); err != nil {
		t.Fatalf("unexpected store create error: %v", err)
	}

	getResponse := makeRequest(t, srv, http.MethodGet, "/containers/hello", nil)

	requireStatus(t, getResponse.Code, http.StatusOK)
	assertContentType(t, getResponse.Header(), "application/json")

	var got container.Container
	decodeJSON(t, getResponse.Body, &got)

	want := container.Container{
		Name:      "hello",
		State:     container.StateRunning,
		CreatedAt: fixedTime,
	}

	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestCreateContainerOmitsState(t *testing.T) {
	response := makeRequest(t, newTestServer(t), http.MethodPost, "/containers", createContainerRequest{
		Name:  "hello",
		Image: "docker.io/library/alpine:latest",
	})

	requireStatus(t, response.Code, http.StatusCreated)

	// we need a map here because decoding into Container cannot tell whether state was omitted or returned as an empty string.
	var got map[string]any
	decodeJSON(t, response.Body, &got)

	if _, ok := got["state"]; ok {
		t.Error("response contains state, want it omitted")
	}
}
