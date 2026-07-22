package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// We test some behaviour already implemented by encoding/json, even though
// normally we should not test code that we do not own. This ensures that our
// JSON function preserves that behaviour while also enforcing the additional rules we added.
func TestServerReadJSON(t *testing.T) {
	type payload struct {
		Name string `json:"name"`
	}

	validTests := []struct {
		name        string
		contentType string
		input       string
		want        payload
	}{
		{"decodes valid JSON", "application/json", `{"name":"hello"}`, payload{Name: "hello"}},
		{"accepts a charset parameter", "application/json; charset=utf-8", `{"name":"hello"}`, payload{Name: "hello"}},
	}

	srv := newTestServer(t)

	for _, tt := range validTests {
		t.Run(tt.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(tt.input))
			request.Header.Set("Content-Type", tt.contentType)
			response := httptest.NewRecorder()

			var got payload
			if ok := srv.readJSON(response, request, &got); !ok {
				t.Fatalf("expected JSON to be accepted, got status %d", response.Code)
			}
			if got != tt.want {
				t.Errorf("got %+v, want %+v", got, tt.want)
			}
		})
	}

	invalidTests := []struct {
		name        string
		contentType string
		input       string
		wantStatus  int
	}{
		{"rejects malformed JSON", "application/json", `{"name":`, http.StatusBadRequest},
		{"rejects unknown fields", "application/json", `{"name":"hello","extra":true}`, http.StatusBadRequest},
		{"rejects multiple JSON values", "application/json", `{"name":"hello"}{}`, http.StatusBadRequest},
		{"rejects a non-JSON content type", "text/plain", `{"name":"hello"}`, http.StatusUnsupportedMediaType},
		{"rejects a missing content type", "", `{"name":"hello"}`, http.StatusUnsupportedMediaType},
	}

	for _, tt := range invalidTests {
		t.Run(tt.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(tt.input))
			if tt.contentType != "" {
				request.Header.Set("Content-Type", tt.contentType)
			}
			response := httptest.NewRecorder()

			var got payload
			if ok := srv.readJSON(response, request, &got); ok {
				t.Fatal("expected JSON to be rejected")
			}
			assertStatus(t, response.Code, tt.wantStatus)
		})
	}
}

func TestServerWriteJSON(t *testing.T) {
	type payload struct {
		Name string `json:"name"`
	}

	srv := newTestServer(t)

	t.Run("writes a JSON response", func(t *testing.T) {
		response := httptest.NewRecorder()
		want := payload{Name: "hello"}

		srv.writeJSON(response, http.StatusCreated, want)

		assertStatus(t, response.Code, http.StatusCreated)
		assertContentType(t, response.Header(), "application/json")

		var got payload
		decodeJSON(t, response.Body, &got)
		if got != want {
			t.Errorf("got %+v, want %+v", got, want)
		}
	})

	t.Run("sends an internal server error when marshaling fails", func(t *testing.T) {
		response := httptest.NewRecorder()

		// Channels cannot be marshaled to JSON by encoding/json.
		srv.writeJSON(response, http.StatusCreated, make(chan int))

		assertStatus(t, response.Code, http.StatusInternalServerError)
	})
}
