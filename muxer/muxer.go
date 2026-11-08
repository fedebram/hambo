// Package muxer multiplexes bidirectional streams over a single connection.
package muxer

import (
	"errors"
	"io"
	"net"
	"sync"
)

var ErrSessionClosed = errors.New("muxer: session closed")

const acceptBacklog = 64

// Session is a single connection between two peers.
type Session struct {
	conn   net.Conn
	err    error
	sendCh chan frame

	mu      sync.Mutex
	streams map[uint32]*Stream

	acceptCh chan *Stream

	done      chan struct{} // closed when the session ends
	closeOnce sync.Once
}

type frame struct {
	id      uint32
	payload []byte
}

func NewSession(conn net.Conn) *Session {
	s := &Session{
		conn:     conn,
		sendCh:   make(chan frame),
		streams:  make(map[uint32]*Stream),
		acceptCh: make(chan *Stream, acceptBacklog),
		done:     make(chan struct{}),
	}
	go s.readLoop()
	go s.writeLoop()
	return s
}

// OpenStream creates stream id on this side only, nothing is sent.
func (s *Session) OpenStream(id uint32) *Stream {
	st := newStream(id, s)

	s.mu.Lock()
	s.streams[id] = st
	s.mu.Unlock()

	return st
}

// Accept waits for the peer to open a new stream and returns it.
func (s *Session) Accept() (*Stream, error) {
	select {
	case st := <-s.acceptCh:
		return st, nil
	case <-s.done:
		err := s.err
		if err == io.EOF {
			err = ErrSessionClosed
		}
		return nil, err
	}
}

func (s *Session) readLoop() {
	for {
		id, payload, err := readFrame(s.conn)
		if err != nil {
			s.closeWithErr(err)
			return
		}

		s.mu.Lock()
		st, ok := s.streams[id]
		if !ok {
			st = newStream(id, s)
			s.streams[id] = st
		}
		s.mu.Unlock()

		if !ok {
			select {
			case s.acceptCh <- st:
			case <-s.done:
				return
			}
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
	for {
		select {
		case f := <-s.sendCh:
			if err := writeFrame(s.conn, f.id, f.payload); err != nil {
				s.closeWithErr(err)
				return
			}
		case <-s.done:
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

// Close ends the session and closes the underlying connection.
func (s *Session) Close() error {
	s.closeWithErr(ErrSessionClosed)
	return nil
}
