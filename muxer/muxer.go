// Package muxer multiplexes bidirectional streams over a single connection.
package muxer

import (
	"errors"
	"net"
	"sync"
)

var ErrSessionClosed = errors.New("muxer: session closed")

// Session is a single connection between two peers.
type Session struct {
	conn    net.Conn
	err     error
	sendCh  chan frame
	streams map[uint32]*Stream

	done      chan struct{} // closed when the session ends
	closeOnce sync.Once
}

type frame struct {
	id      uint32
	payload []byte
}

func NewSession(conn net.Conn) *Session {
	s := &Session{
		conn:    conn,
		sendCh:  make(chan frame),
		streams: make(map[uint32]*Stream),
		done:    make(chan struct{}),
	}
	go s.readLoop()
	go s.writeLoop()
	return s
}

func (s *Session) readLoop() {
	for {
		id, payload, err := readFrame(s.conn)
		if err != nil {
			s.closeWithErr(err)
			return
		}

		st, ok := s.streams[id]
		if !ok {
			continue
		}

		st.mu.Lock()
		st.buf.Write(payload)
		st.mu.Unlock()

		select {
		case st.notify <- struct{}{}:
		default:
		}
	}
}

func (s *Session) writeLoop() {
	for f := range s.sendCh {
		if err := writeFrame(s.conn, f.id, f.payload); err != nil {
			s.closeWithErr(err)
			return
		}
	}
}

// closeWithErr ends the session once, keeping the first reason.
func (s *Session) closeWithErr(err error) {
	s.closeOnce.Do(func() {
		s.err = err
		close(s.done)
		s.conn.Close()
	})
}

// isClosed reports whether the session has ended, without blocking.
func (s *Session) isClosed() bool {
	select {
	case <-s.done:
		return true
	default:
		return false
	}
}

// Read reads from the underlying connection.
func (s *Session) Read(p []byte) (int, error) {
	return s.conn.Read(p)
}

// Write writes to the underlying connection.
func (s *Session) Write(p []byte) (int, error) {
	return s.conn.Write(p)
}

// Close ends the session and closes the underlying connection.
func (s *Session) Close() error {
	s.closeWithErr(ErrSessionClosed)
	return nil
}
