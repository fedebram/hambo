// Package muxer multiplexes bidirectional streams over a single connection.
package muxer

import "net"

// Session is a single connection between two peers.
type Session struct {
	conn net.Conn
}

func NewSession(conn net.Conn) *Session {
	return &Session{conn: conn}
}

// Read reads from the underlying connection.
func (s *Session) Read(p []byte) (int, error) {
	return s.conn.Read(p)
}

// Write writes to the underlying connection.
func (s *Session) Write(p []byte) (int, error) {
	return s.conn.Write(p)
}

// Close closes the underlying connection.
func (s *Session) Close() error {
	return s.conn.Close()
}
