package api

import (
	"net/http"
	"time"

	publicapi "github.com/fedebram/hambo/api"
	"github.com/fedebram/hambo/internal/token"
)

const (
	maxTokenRequestSize = 1024
	defaultTokenTTL     = 15 * time.Minute
	maxTokenTTL         = 24 * time.Hour
)

func (srv *server) createTokenHandler(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxTokenRequestSize)

	var request publicapi.CreateTokenRequest
	if !srv.readJSON(w, r, &request) {
		return
	}

	ttl := defaultTokenTTL
	if request.TTL != "" {
		var err error
		ttl, err = time.ParseDuration(request.TTL)
		if err != nil {
			srv.writeValidationErrorJSON(w, map[string]string{
				"ttl": "must be a duration such as 15m or 1h",
			})
			return
		}
	}
	if ttl <= 0 || ttl > maxTokenTTL {
		srv.writeValidationErrorJSON(w, map[string]string{
			"ttl": "must be greater than zero and no more than 24h",
		})
		return
	}

	generated, err := token.Generate(srv.ca.Certificate)
	if err != nil {
		srv.logger.Error("generate token", "error", err)
		srv.writeErrorJSON(w, http.StatusInternalServerError, "could not create token")
		return
	}

	expiresAt := time.Now().UTC().Add(ttl)
	if err := srv.tokens.Create(generated.SecretHash(), token.Metadata{
		ExpiresAt: expiresAt,
	}); err != nil {
		srv.logger.Error("store token", "error", err)
		srv.writeErrorJSON(w, http.StatusInternalServerError, "could not create token")
		return
	}

	w.Header().Set("Cache-Control", "no-store")
	srv.writeJSON(w, http.StatusCreated, publicapi.CreateTokenResponse{
		Token:     generated.Format(),
		ExpiresAt: expiresAt,
	})
}
