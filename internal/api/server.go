package api

import (
	"log/slog"
	"net/http"
	"time"
)

type serverConfig struct {
	now    func() time.Time
	logger *slog.Logger
}

type server struct {
	mux    *http.ServeMux
	now    func() time.Time
	logger *slog.Logger
}

// Inspired by https://grafana.com/blog/how-i-write-http-services-in-go-after-13-years/

func NewServer(options ...Option) http.Handler {
	return newServer(options...)
}

func newServer(options ...Option) *server {
	config := serverConfig{now: time.Now, logger: slog.Default()}
	for _, option := range options {
		option(&config)
	}

	srv := &server{
		mux:    http.NewServeMux(),
		now:    config.now,
		logger: config.logger,
	}
	srv.addRoutes()

	return srv
}

func (srv *server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	srv.mux.ServeHTTP(w, r)
}
