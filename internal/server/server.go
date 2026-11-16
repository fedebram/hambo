package server

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"time"

	"github.com/fedebram/hambo/internal/rpc"
	"github.com/fedebram/hambo/internal/store"
	"github.com/fedebram/hambo/internal/transport"
	"github.com/flynn/noise"
)

// how long an unknown client may stay connected
const unknownTimeout = 30 * time.Second

// Server serves the admins and the workers of the cluster.
type Server struct {
	key    noise.DHKey
	cidr   netip.Prefix
	st     *store.Store
	reg    *registry
	router *rpc.Router
}

func New(key noise.DHKey, cidr netip.Prefix, st *store.Store) *Server {
	s := &Server{
		key:  key,
		cidr: cidr,
		st:   st,
		reg:  newRegistry(),
	}
	s.router = s.routes()
	return s
}

// Serve accepts connections on ln and serves each one in its own goroutine.
func (s *Server) Serve(ln net.Listener) error {
	for {
		conn, err := ln.Accept()
		if err != nil {
			return err
		}
		go s.serveConn(conn)
	}
}

// serveConn runs the handshake and serves rpc until the session ends.
func (s *Server) serveConn(conn net.Conn) {
	sess, pub, err := transport.Accept(conn, s.key)
	if err != nil {
		fmt.Println("handshake failed:", conn.RemoteAddr(), err)
		return
	}
	defer sess.Close()

	ctx := context.Background()
	role, err := s.st.Role(ctx, pub)
	if err != nil {
		fmt.Println("role lookup failed:", conn.RemoteAddr(), err)
		return
	}

	switch role {
	case store.RoleUnknown:
		// an unknown client gets at most 30 seconds to call join (the only
		// method it may call), then we close the session, so it can't keep
		// one open forever.
		time.AfterFunc(unknownTimeout, func() { sess.Close() })
	case store.RoleWorker:
		s.reg.add(pub, sess)
		defer s.reg.remove(pub, sess)
		go s.sendWGConfig(ctx, pub, sess)
	}

	s.router.Serve(withCaller(ctx, caller{PubKey: pub}), sess)
}
