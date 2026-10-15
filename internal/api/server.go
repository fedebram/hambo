package api

import (
	"log/slog"
	"net/http"

	"github.com/fedebram/hambo/internal/pki"
	"github.com/fedebram/hambo/internal/store"
	"github.com/fedebram/hambo/internal/token"
)

type server struct {
	mux    *http.ServeMux
	logger *slog.Logger
	tokens *store.Collection[token.Metadata]
	ca     *pki.CA
}

// Inspired by https://grafana.com/blog/how-i-write-http-services-in-go-after-13-years/

func NewServer(tokens *store.Collection[token.Metadata], ca *pki.CA, options ...ServerOption) http.Handler {
	return newServer(tokens, ca, options...)
}

func newServer(tokens *store.Collection[token.Metadata], ca *pki.CA, options ...ServerOption) *server {
	if tokens == nil {
		panic("api: token collection cannot be nil")
	}
	if ca == nil {
		panic("api: CA certificate cannot be nil")
	}

	srv := &server{
		mux:    http.NewServeMux(),
		logger: slog.Default(),
		tokens: tokens,
		ca:     ca,
	}

	for _, option := range options {
		option(srv)
	}

	srv.addRoutes()

	return srv
}

func (srv *server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	srv.mux.ServeHTTP(w, r)
}
