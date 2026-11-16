package server

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"time"

	"github.com/fedebram/hambo/internal/api"
	"github.com/fedebram/hambo/internal/backoff"
	"github.com/fedebram/hambo/internal/queue"
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

	queue   *queue.MemoryQueue
	backoff *backoff.Backoff
}

// how many nodes are updated concurrently
const updaters = 4

// New returns a server that uses key for the handshake, gives the nodes
// subnets of cidr, and keeps its data in st.
func New(key noise.DHKey, cidr netip.Prefix, st *store.Store) *Server {
	s := &Server{
		key:     key,
		cidr:    cidr,
		st:      st,
		reg:     newRegistry(),
		queue:   queue.NewMemoryQueue(),
		backoff: backoff.NewBackoff(time.Second, time.Minute),
	}
	s.router = s.routes()
	return s
}

// Serve accepts connections on ln and serves each one in its own goroutine.
// It returns when ln is closed.
func (s *Server) Serve(ln net.Listener) error {
	// the updaters wait on the queue and stop when it shuts down
	for range updaters {
		go s.runUpdates()
	}
	defer s.queue.Shutdown()

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

		// we get the worker endpoint: where the other nodes reach its wg0.
		endpoint, err := wgEndpoint(conn)
		if err != nil {
			fmt.Println("endpoint:", workerName(pub), err)
			return
		}
		changed, err := s.st.SetEndpoint(ctx, pub, endpoint)
		if err != nil {
			fmt.Println("endpoint:", workerName(pub), err)
			return
		}

		// when changed is true it means that the endpoint changed or also
		// the node is connected for the first time.
		// we update all the nodes in the cluster to reflect the new endpoint.
		//
		// (previously I wondered if wireguard roaming can make this automatically,
		//  but in fact it is not possible)
		if changed {
			s.updateAllNodes()
		} else {
			// mmmmh the name is not that good....
			s.updateLater(pub)
		}
	}

	s.router.Serve(withCaller(ctx, caller{PubKey: pub}), sess)
}

// wgEndpoint returns the address used by the worker to establish this conn
// combined with the wireguard port.
func wgEndpoint(conn net.Conn) (netip.AddrPort, error) {
	remote, err := netip.ParseAddrPort(conn.RemoteAddr().String())
	if err != nil {
		return netip.AddrPort{}, err
	}
	return netip.AddrPortFrom(remote.Addr().Unmap(), api.WGPort), nil
}
