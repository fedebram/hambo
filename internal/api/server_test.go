package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"mime"
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
		request := httptest.NewRequest(http.MethodGet, "/health", nil)
		response := httptest.NewRecorder()

		store := container.NewMemoryStore()
		srv := NewServer(store)
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

func TestCreateContainer(t *testing.T) {
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

	store := container.NewMemoryStore()
	srv := NewServer(store, WithClock(fakeClock))

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := jsonBody(t, createContainerRequest{Name: tt.containerName})
			request := httptest.NewRequest(http.MethodPost, "/containers", body)
			request.Header.Set("Content-Type", "application/json")
			response := httptest.NewRecorder()

			srv.ServeHTTP(response, request)

			assertStatus(t, response.Code, http.StatusCreated)
			assertContentType(t, response.Header(), "application/json")

			var got container.Container
			decodeJSON(t, response.Body, &got)

			want := container.Container{
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

	store := container.NewMemoryStore()
	NewServer(store).ServeHTTP(response, request)

	assertStatus(t, response.Code, http.StatusBadRequest)
}

func TestGetMissingContainerReturnsNotFound(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/containers/missing", nil)
	response := httptest.NewRecorder()

	store := container.NewMemoryStore()
	NewServer(store).ServeHTTP(response, request)

	assertStatus(t, response.Code, http.StatusNotFound)
}

func TestContainerStoreFailuresReturnInternalServerError(t *testing.T) {
	t.Run("GET", func(t *testing.T) {
		request := httptest.NewRequest(http.MethodGet, "/containers/hello", nil)
		response := httptest.NewRecorder()
		store := failingStore{err: errors.New("store unavailable")}

		NewServer(store).ServeHTTP(response, request)

		assertStatus(t, response.Code, http.StatusInternalServerError)
	})

	t.Run("POST", func(t *testing.T) {
		body := jsonBody(t, createContainerRequest{Name: "hello"})
		request := httptest.NewRequest(http.MethodPost, "/containers", body)
		request.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		store := failingStore{err: errors.New("store unavailable")}

		NewServer(store).ServeHTTP(response, request)

		assertStatus(t, response.Code, http.StatusInternalServerError)
	})
}

func TestCreateAndGetContainer(t *testing.T) {
	fixedTime := time.Date(2026, time.July, 19, 15, 0, 0, 0, time.UTC)
	store := container.NewMemoryStore()
	srv := NewServer(store, WithClock(func() time.Time {
		return fixedTime
	}))

	body := jsonBody(t, createContainerRequest{Name: "hello"})
	createRequest := httptest.NewRequest(http.MethodPost, "/containers", body)
	createRequest.Header.Set("Content-Type", "application/json")
	createResponse := httptest.NewRecorder()

	srv.ServeHTTP(createResponse, createRequest)

	requireStatus(t, createResponse.Code, http.StatusCreated)

	getRequest := httptest.NewRequest(http.MethodGet, "/containers/hello", nil)
	getResponse := httptest.NewRecorder()

	srv.ServeHTTP(getResponse, getRequest)

	requireStatus(t, getResponse.Code, http.StatusOK)
	assertContentType(t, getResponse.Header(), "application/json")

	var got container.Container
	decodeJSON(t, getResponse.Body, &got)

	want := container.Container{
		Name:      "hello",
		CreatedAt: fixedTime,
	}

	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestCreateContainerRejectsDuplicateName(t *testing.T) {
	firstTime := time.Date(2026, time.July, 19, 15, 0, 0, 0, time.UTC)
	clockCalls := 0
	store := container.NewMemoryStore()
	srv := NewServer(store, WithClock(func() time.Time {
		current := firstTime.Add(time.Duration(clockCalls) * time.Minute)
		clockCalls++
		return current
	}))

	firstBody := jsonBody(t, createContainerRequest{Name: "hello"})
	firstRequest := httptest.NewRequest(http.MethodPost, "/containers", firstBody)
	firstRequest.Header.Set("Content-Type", "application/json")
	firstResponse := httptest.NewRecorder()

	srv.ServeHTTP(firstResponse, firstRequest)

	requireStatus(t, firstResponse.Code, http.StatusCreated)

	duplicateBody := jsonBody(t, createContainerRequest{Name: "hello"})
	duplicateRequest := httptest.NewRequest(http.MethodPost, "/containers", duplicateBody)
	duplicateRequest.Header.Set("Content-Type", "application/json")
	duplicateResponse := httptest.NewRecorder()

	srv.ServeHTTP(duplicateResponse, duplicateRequest)

	assertStatus(t, duplicateResponse.Code, http.StatusConflict)

	getRequest := httptest.NewRequest(http.MethodGet, "/containers/hello", nil)
	getResponse := httptest.NewRecorder()

	srv.ServeHTTP(getResponse, getRequest)

	requireStatus(t, getResponse.Code, http.StatusOK)

	var got container.Container
	decodeJSON(t, getResponse.Body, &got)

	want := container.Container{
		Name:      "hello",
		CreatedAt: firstTime,
	}

	if got != want {
		t.Errorf("got %+v, want original %+v", got, want)
	}
}

// failingStore lets us test how the api handles unexpected store errors.
type failingStore struct {
	err error
}

func (s failingStore) Create(container.Container) error {
	return s.err
}

func (s failingStore) Get(string) (container.Container, error) {
	return container.Container{}, s.err
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

func requireStatus(t *testing.T, got, want int) {
	t.Helper()

	if got != want {
		t.Fatalf(
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
