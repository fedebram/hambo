package api

import (
	"fmt"
	"log/slog"
	"net/http"

	"github.com/fedebram/hambo/internal/container"
	"github.com/fedebram/hambo/internal/node"
	"github.com/fedebram/hambo/internal/pki"
	"github.com/fedebram/hambo/internal/queue"
	"github.com/fedebram/hambo/internal/store"
	"github.com/fedebram/hambo/internal/token"
)

type server struct {
	mux        *http.ServeMux
	logger     *slog.Logger
	db         *store.Store
	tokens     *store.Collection[token.Metadata]
	nodes      *store.Collection[node.Node]
	containers *store.Collection[container.Container]
	ca         *pki.CA

	schedulingQueue queue.Queue
}

// Inspired by https://grafana.com/blog/how-i-write-http-services-in-go-after-13-years/

func NewServer(
	db *store.Store,
	ca *pki.CA,
	schedulingQueue queue.Queue,
	nodes *store.Collection[node.Node],
	containers *store.Collection[container.Container],
	options ...ServerOption,
) (http.Handler, error) {
	srv, err := newServer(db, ca, schedulingQueue, nodes, containers, options...)
	if err != nil {
		return nil, err
	}
	return srv, nil
}

func newServer(
	db *store.Store,
	ca *pki.CA,
	schedulingQueue queue.Queue,
	nodes *store.Collection[node.Node],
	containers *store.Collection[container.Container],
	options ...ServerOption,
) (*server, error) {
	if db == nil {
		panic("api: db cannot be nil")
	}
	if ca == nil {
		panic("api: CA certificate cannot be nil")
	}
	if schedulingQueue == nil {
		panic("api: scheduling queue cannot be nil")
	}
	if nodes == nil {
		panic("api: nodes collection cannot be nil")
	}
	if containers == nil {
		panic("api: containers collection cannot be nil")
	}

	tokens, err := store.NewCollection[token.Metadata](db, "tokens")
	if err != nil {
		return nil, fmt.Errorf("open token collection: %w", err)
	}
	srv := &server{
		mux:             http.NewServeMux(),
		logger:          slog.Default(),
		db:              db,
		tokens:          tokens,
		nodes:           nodes,
		containers:      containers,
		ca:              ca,
		schedulingQueue: schedulingQueue,
	}

	for _, option := range options {
		option(srv)
	}

	srv.addRoutes()

	return srv, nil
}

func (srv *server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	srv.mux.ServeHTTP(w, r)
}
