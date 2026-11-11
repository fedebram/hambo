package noiseconn

import (
	"net"

	"github.com/flynn/noise"
)

// A Conn represents a secured connection.
// It implements the net.Conn interface.
type Conn struct {
	net.Conn
	send    *noise.CipherState
	recv    *noise.CipherState
	pending []byte
}

// Write encrypts p and sends it.
func (c *Conn) Write(p []byte) (int, error) {
	n := 0
	for len(p) > 0 {
		chunk := p[:min(len(p), maxPlain)]

		ct, err := c.send.Encrypt(nil, nil, chunk)
		if err != nil {
			return n, err
		}
		if err := writeMsg(c.Conn, ct); err != nil {
			return n, err
		}

		n += len(chunk)
		p = p[len(chunk):]
	}
	return n, nil
}

// Read returns decrypted data.
func (c *Conn) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	if len(c.pending) == 0 {
		ct, err := readMsg(c.Conn)
		if err != nil {
			return 0, err
		}
		pt, err := c.recv.Decrypt(nil, nil, ct)
		if err != nil {
			return 0, err
		}
		c.pending = pt
	}

	n := copy(p, c.pending)
	c.pending = c.pending[n:]
	return n, nil
}

// Client runs the handshake as the client and returns an encrypted conn.
func Client(conn net.Conn, local noise.DHKey, serverPub []byte) (*Conn, error) {
	send, recv, err := clientHandshake(conn, local, serverPub, nil, nil)
	if err != nil {
		return nil, err
	}
	return &Conn{Conn: conn, send: send, recv: recv}, nil
}

// Join runs the handshake as a new client with a join token,
// and returns an encrypted conn.
func Join(conn net.Conn, local noise.DHKey, serverPub, tokenID, secret []byte) (*Conn, error) {
	send, recv, err := clientHandshake(conn, local, serverPub, tokenID, secret)
	if err != nil {
		return nil, err
	}
	return &Conn{Conn: conn, send: send, recv: recv}, nil
}

// Server runs the handshake as the server and returns an encrypted conn
// and what it learned about the client (Peer). allowed checks known clients.
// joinSecret returns a join token's secret, or nil if there's no such token.
func Server(conn net.Conn, local noise.DHKey, allowed func(pub []byte) bool, joinSecret func(id []byte) []byte) (*Conn, Peer, error) {
	send, recv, peer, err := serverHandshake(conn, local, allowed, joinSecret)
	if err != nil {
		return nil, Peer{}, err
	}
	return &Conn{Conn: conn, send: send, recv: recv}, peer, nil
}
