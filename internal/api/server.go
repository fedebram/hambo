package api

import (
	"crypto/x509"
	"log/slog"
	"net/http"

	"github.com/fedebram/hambo/internal/store"
	"github.com/fedebram/hambo/internal/token"
)

type server struct {
	mux    *http.ServeMux
	logger *slog.Logger
	tokens *store.Collection[token.Metadata]
	caCert *x509.Certificate
}

// Inspired by https://grafana.com/blog/how-i-write-http-services-in-go-after-13-years/

func NewServer(tokens *store.Collection[token.Metadata], caCert *x509.Certificate, options ...ServerOption) http.Handler {
	return newServer(tokens, caCert, options...)
}

func newServer(tokens *store.Collection[token.Metadata], caCert *x509.Certificate, options ...ServerOption) *server {
	if tokens == nil {
		panic("api: token collection cannot be nil")
	}
	if caCert == nil {
		panic("api: CA certificate cannot be nil")
	}

	srv := &server{
		mux:    http.NewServeMux(),
		logger: slog.Default(),
		tokens: tokens,
		caCert: caCert,
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
