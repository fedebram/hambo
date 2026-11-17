package server

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"time"

	"github.com/fedebram/hambo/internal/api"
	"github.com/fedebram/hambo/internal/muxer"
	"github.com/fedebram/hambo/internal/store"
	"github.com/fedebram/hambo/internal/transport"
)

// how long an unknown client may stay connected
const unknownTimeout = 30 * time.Second

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
		// an unknown client may only call join: it gets 30 seconds,
		// so it can't keep a session open forever.
		time.AfterFunc(unknownTimeout, func() { sess.Close() })
	case store.RoleNode:
		s.sessions.add(pub, sess)
		defer s.sessions.remove(pub, sess)

		changed, err := s.saveEndpoint(ctx, pub, conn)
		if err != nil {
			fmt.Println("endpoint:", workerName(pub), err)
			return
		}
		s.onNodeConnected(pub, changed)
	}

	s.router.Serve(withCaller(ctx, caller{PubKey: pub}), sess)
}

// disconnect ends a node's session.
func (s *Server) disconnect(sess *muxer.Session, name, reason string) {
	fmt.Println("disconnect", name+":", reason)
	sess.Close()
}

// saveEndpoint saves where the other nodes reach this node's wg0: the
// address it connected from, with the wireguard port. changed is true
// if it is new or different.
func (s *Server) saveEndpoint(ctx context.Context, pub []byte, conn net.Conn) (changed bool, err error) {
	remote, err := netip.ParseAddrPort(conn.RemoteAddr().String())
	if err != nil {
		return false, err
	}
	endpoint := netip.AddrPortFrom(remote.Addr().Unmap(), api.WGPort)
	return s.st.SetEndpoint(ctx, pub, endpoint.String())
}
