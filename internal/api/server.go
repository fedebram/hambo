package api

import (
	"log/slog"
	"net/http"
)

type server struct {
	mux    *http.ServeMux
	logger *slog.Logger
}

// Inspired by https://grafana.com/blog/how-i-write-http-services-in-go-after-13-years/

func NewServer(options ...ServerOption) http.Handler {
	return newServer(options...)
}

func newServer(options ...ServerOption) *server {
	srv := &server{
		mux:    http.NewServeMux(),
		logger: slog.Default(),
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
