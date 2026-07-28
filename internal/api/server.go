package api

import (
	"log/slog"
	"net/http"

	"github.com/fedebram/hambo/internal/container"
)

type server struct {
	mux     *http.ServeMux
	service *container.Service
	logger  *slog.Logger
}

// Inspired by https://grafana.com/blog/how-i-write-http-services-in-go-after-13-years/

func NewServer(service *container.Service, options ...ServerOption) http.Handler {
	return newServer(service, options...)
}

func newServer(service *container.Service, options ...ServerOption) *server {
	srv := &server{
		mux:     http.NewServeMux(),
		service: service,
		logger:  slog.Default(),
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
