package transport

import (
	"net"

	"github.com/fedebram/hambo/internal/muxer"
	"github.com/fedebram/hambo/internal/noiseconn"
	"github.com/flynn/noise"
)

// Dial connects via tcp to the hambo server at addr and returns a session.
func Dial(addr string, key noise.DHKey, serverPub []byte) (*muxer.Session, error) {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return nil, err
	}
	nc, err := noiseconn.Client(conn, key, serverPub)
	if err != nil {
		conn.Close()
		return nil, err
	}
	return muxer.Client(nc, muxer.Config{}), nil
}

// Accept runs the server side of the handshake on conn, which a client
// opened, and returns a session and the client's verified public key.
func Accept(conn net.Conn, key noise.DHKey) (*muxer.Session, []byte, error) {
	nc, peer, err := noiseconn.Server(conn, key)
	if err != nil {
		conn.Close()
		return nil, nil, err
	}
	return muxer.Server(nc, muxer.Config{}), peer, nil
}
