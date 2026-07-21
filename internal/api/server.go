package api

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/fedebram/hambo/internal/container"
)

type server struct {
	mux    *http.ServeMux
	store  container.Store
	now    func() time.Time
	logger *slog.Logger
}

// Inspired by https://grafana.com/blog/how-i-write-http-services-in-go-after-13-years/

func NewServer(store container.Store, options ...Option) http.Handler {
	return newServer(store, options...)
}

func newServer(store container.Store, options ...Option) *server {
	srv := &server{
		mux:    http.NewServeMux(),
		store:  store,
		now:    time.Now,
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
