package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	publicapi "github.com/fedebram/hambo/api"
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
		wantCode    string
		wantMessage string
	}{
		{"rejects malformed JSON", "application/json", `{"name":`, http.StatusBadRequest, publicapi.ErrorCodeInvalidJSON, "request body contains invalid JSON"},
		{"rejects unknown fields", "application/json", `{"name":"hello","extra":true}`, http.StatusBadRequest, publicapi.ErrorCodeInvalidJSON, "request body contains invalid JSON"},
		{"rejects multiple JSON values", "application/json", `{"name":"hello"}{}`, http.StatusBadRequest, publicapi.ErrorCodeInvalidJSON, "request body contains invalid JSON"},
		{"rejects a non-JSON content type", "text/plain", `{"name":"hello"}`, http.StatusUnsupportedMediaType, publicapi.ErrorCodeUnsupportedMediaType, "Content-Type must be application/json"},
		{"rejects a missing content type", "", `{"name":"hello"}`, http.StatusUnsupportedMediaType, publicapi.ErrorCodeUnsupportedMediaType, "Content-Type must be application/json"},
	}

	for _, tt := range invalidTests {
		t.Run(tt.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(tt.input))
			if tt.contentType != "" {
				request.Header.Set("Content-Type", tt.contentType)
			}
			response := httptest.NewRecorder()

			var decoded payload
			if ok := srv.readJSON(response, request, &decoded); ok {
				t.Fatal("expected JSON to be rejected")
			}
			assertStatus(t, response.Code, tt.wantStatus)
			assertContentType(t, response.Header(), "application/json")

			var got publicapi.ErrorResponse
			decodeJSON(t, response.Body, &got)

			if got.Code != tt.wantCode {
				t.Errorf("got error code %q, want %q", got.Code, tt.wantCode)
			}
			if got.Message != tt.wantMessage {
				t.Errorf("got error message %q, want %q", got.Message, tt.wantMessage)
			}
			if len(got.Fields) != 0 {
				t.Errorf("got fields %+v, want none", got.Fields)
			}
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

func TestJSONStream(t *testing.T) {
	type payload struct {
		Name string `json:"name"`
	}

	response := httptest.NewRecorder()

	stream, ok := startJSONStream(response, http.StatusOK)
	if !ok {
		t.Fatal("expected response writer to support streaming")
	}

	assertStatus(t, response.Code, http.StatusOK)
	assertContentType(t, response.Header(), "application/x-ndjson")

	if err := stream.Write(payload{Name: "first"}); err != nil {
		t.Fatalf("write first event: %v", err)
	}

	want := "{\"name\":\"first\"}\n"
	if got := response.Body.String(); got != want {
		t.Errorf("after first write: got body %q, want %q", got, want)
	}

	if err := stream.Write(payload{Name: "second"}); err != nil {
		t.Fatalf("write second event: %v", err)
	}

	want = "{\"name\":\"first\"}\n{\"name\":\"second\"}\n"
	if got := response.Body.String(); got != want {
		t.Errorf("after second write: got body %q, want %q", got, want)
	}
}

func TestJSONStreamWriteError(t *testing.T) {
	response := httptest.NewRecorder()

	stream, ok := startJSONStream(response, http.StatusOK)
	if !ok {
		t.Fatal("expected response writer to support streaming")
	}

	if err := stream.Write(make(chan int)); err == nil {
		t.Fatal("expected write to fail")
	}
}
