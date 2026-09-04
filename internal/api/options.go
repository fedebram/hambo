package api

import (
	"log/slog"
)

// This option pattern is loosely based on containerd go-client client-opts.
// https://github.com/containerd/containerd/blob/main/docs/client-opts.md

// ServerOption represents a function that modifies or extends the server.
type ServerOption func(*server)

func WithLogger(logger *slog.Logger) ServerOption {
	if logger == nil {
		panic("api: logger cannot be nil")
	}

	return func(srv *server) {
		srv.logger = logger
	}
}

func WithImageService(service imageService) ServerOption {
	if service == nil {
		panic("api: image service cannot be nil")
	}

	return func(srv *server) {
		srv.imageService = service
	}
}
