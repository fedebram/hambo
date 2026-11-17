package server

import (
	"net"
	"net/netip"
	"sync"

	"github.com/fedebram/hambo/internal/rpc"
	"github.com/fedebram/hambo/internal/store"
	"github.com/flynn/noise"
)

// Server serves the admins and the nodes of the cluster.
type Server struct {
	key      noise.DHKey
	cidr     netip.Prefix
	st       *store.Store
	router   *rpc.Router
	sessions *sessions

	nodeSyncer *controller
	scheduler  *controller

	joinMu sync.Mutex
}

// New returns a server that uses key for the handshake, gives the nodes
// subnets of cidr, and keeps its data in st.
func New(key noise.DHKey, cidr netip.Prefix, st *store.Store) *Server {
	s := &Server{
		key:      key,
		cidr:     cidr,
		st:       st,
		sessions: newSessions(),
	}
	s.router = s.routes()
	s.nodeSyncer = newController("sync node", 4, s.syncNode)
	s.scheduler = newController("schedule container", 1, s.scheduleContainer)
	return s
}

// Serve accepts connections on ln and serves each one in its own goroutine.
// It returns when ln is closed.
func (s *Server) Serve(ln net.Listener) error {
	s.nodeSyncer.run()
	defer s.nodeSyncer.shutdown()
	s.scheduler.run()
	defer s.scheduler.shutdown()

	for {
		conn, err := ln.Accept()
		if err != nil {
			return err
		}
		go s.serveConn(conn)
	}
}
