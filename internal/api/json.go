package api

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
)

// readJSON is based on https://www.alexedwards.net/blog/how-to-properly-parse-a-json-request-body

// readJSON returns true when decoding succeeds. On failure it writes the appropriate
// HTTP error response and returns false.
func (srv *server) readJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	// we enforce content type application because maybe in the future we want to support other types like cbor
	if err != nil || mediaType != "application/json" {
		http.Error(w, http.StatusText(http.StatusUnsupportedMediaType), http.StatusUnsupportedMediaType)
		return false
	}

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return false
	}

	// The input must contain exactly one JSON value.
	var extra any
	err = dec.Decode(&extra)
	if !errors.Is(err, io.EOF) {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
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
