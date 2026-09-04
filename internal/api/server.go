package api

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/fedebram/hambo/internal/container"
	"github.com/fedebram/hambo/internal/image"
)

type containerService interface {
	Create(container.Container) (container.Container, error)
	Get(name string) (container.Container, error)
	Start(name string) (container.Container, error)
	Stop(name string) (container.Container, error)
	Delete(name string) (container.Container, error)
}

type imageService interface {
	List(context.Context, ...image.ListFilter) ([]image.Image, error)
	Pull(context.Context, string, image.PullProgressFunc) (image.Image, error)
}

type server struct {
	mux          *http.ServeMux
	service      containerService
	imageService imageService
	logger       *slog.Logger
}

// Inspired by https://grafana.com/blog/how-i-write-http-services-in-go-after-13-years/

func NewServer(service containerService, options ...ServerOption) http.Handler {
	return newServer(service, options...)
}

func newServer(service containerService, options ...ServerOption) *server {
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
