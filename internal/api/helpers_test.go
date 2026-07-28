package api

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/fedebram/hambo/internal/container"
)

type fakeClock struct {
	current time.Time
	advance time.Duration
	calls   int
}

// newFakeClock creates a new clock to be used by the server.
// The now function starts at start and advances in time based on the advance paramenter
func newFakeClock(start time.Time, advance time.Duration) *fakeClock {
	return &fakeClock{
		current: start,
		advance: advance,
	}
}

// now is not safe to call concurrently.
func (clock *fakeClock) now() time.Time {
	now := clock.current
	clock.current = clock.current.Add(clock.advance)
	clock.calls++
	return now
}

func newTestServer(t *testing.T, options ...Option) *server {
	t.Helper()

	store := container.NewMemoryStore()

	options = append([]Option{
		WithLogger(slog.New(slog.DiscardHandler)),
	}, options...)

	return newServer(store, options...)
}

func makeRequest(t *testing.T, handler http.Handler, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()

	var requestBody io.Reader
	if body != nil {
		requestBody = jsonBody(t, body)
	}

	request := httptest.NewRequest(method, path, requestBody)
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
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

func (s failingStore) Modify(string, func(*container.Container)) error {
	return s.err
}
