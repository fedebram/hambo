package api

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"

	publicapi "github.com/fedebram/hambo/api"
)

type jsonStream struct {
	encoder *json.Encoder
	flusher http.Flusher
	err     error
}

func startJSONStream(w http.ResponseWriter, statusCode int) (*jsonStream, bool) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		return nil, false
	}

	w.Header().Set("Content-Type", "application/x-ndjson")
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(statusCode)
	flusher.Flush()

	return &jsonStream{
		encoder: json.NewEncoder(w),
		flusher: flusher,
	}, true
}

func (stream *jsonStream) Write(value any) error {
	// not safe to call concurrently!

	if stream.err != nil {
		return stream.err
	}
	if err := stream.encoder.Encode(value); err != nil {
		stream.err = err
		return err
	}

	stream.flusher.Flush()
	return nil
}

func (stream *jsonStream) Err() error {
	return stream.err
}

// readJSON is based on https://www.alexedwards.net/blog/how-to-properly-parse-a-json-request-body

// readJSON returns true when decoding succeeds. On failure it writes the appropriate
// HTTP error response and returns false.
func (srv *server) readJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	// we enforce content type application because maybe in the future we want to support other types like cbor
	if err != nil || mediaType != "application/json" {
		srv.writeErrorJSON(
			w,
			http.StatusUnsupportedMediaType,
			publicapi.ErrorCodeUnsupportedMediaType,
			"Content-Type must be application/json",
		)
		return false
	}

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		srv.writeErrorJSON(
			w,
			http.StatusBadRequest,
			publicapi.ErrorCodeInvalidJSON,
			"request body contains invalid JSON",
		)
		return false
	}

	// The input must contain exactly one JSON value.
	var extra any
	err = dec.Decode(&extra)
	if !errors.Is(err, io.EOF) {
		srv.writeErrorJSON(
			w,
			http.StatusBadRequest,
			publicapi.ErrorCodeInvalidJSON,
			"request body contains invalid JSON",
		)
		return false
	}

	return true
}

func (srv *server) writeJSON(w http.ResponseWriter, statusCode int, data any) {
	body, err := json.Marshal(data)
	if err != nil {
		srv.logger.Error("could not marshal JSON response", "error", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	if _, err := w.Write(body); err != nil {
		// The response headers have already been written, so there is nothing
		// else to do except log the error. Write failed!!
		srv.logger.Error("could not write JSON response", "error", err)
	}
}

func (srv *server) writeErrorJSON(w http.ResponseWriter, statusCode int, code, message string) {
	srv.writeJSON(w, statusCode, publicapi.ErrorResponse{
		Code:    code,
		Message: message,
	})
}
