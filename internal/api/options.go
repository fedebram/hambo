package api

import (
	"log/slog"
	"time"
)

// This option pattern is loosely based on containerd go-client client-opts.
// https://github.com/containerd/containerd/blob/main/docs/client-opts.md

// Option represents a function that modifies or extends the server.
type Option func(*server)

// WithClock configures the function used by the server to get the current time.
func WithClock(now func() time.Time) Option {
	if now == nil {
		panic("api: clock cannot be nil")
	}

	return func(srv *server) {
		srv.now = now
	}
}

func WithLogger(logger *slog.Logger) Option {
	if logger == nil {
		panic("api: logger cannot be nil")
	}

	return func(srv *server) {
		srv.logger = logger
	}
}
