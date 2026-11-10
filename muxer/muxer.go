// Package muxer multiplexes bidirectional streams over a single connection.
package muxer

import (
	"errors"
	"net"
	"sync"
)

var (
	ErrSessionClosed = errors.New("muxer: session closed")
	ErrLabelTooLarge = errors.New("muxer: label too large")
)

const acceptBacklog = 64

// Session is a single connection between two peers.
type Session struct {
	conn   net.Conn
	sendCh chan frame

	mu      sync.Mutex
	streams map[uint32]*Stream
	nextID  uint32

	acceptCh chan *Stream

	done      chan struct{} // closed when the session ends
	closeOnce sync.Once
}

type frame struct {
	flags   uint16
	id      uint32
	payload []byte
}

// Client starts a session on the client side. It opens streams with odd ids.
func Client(conn net.Conn) *Session {
	return newSession(conn, 1)
}

// Server starts a session on the server side. It opens streams with even ids.
func Server(conn net.Conn) *Session {
	return newSession(conn, 2)
}

func newSession(conn net.Conn, firstID uint32) *Session {
	s := &Session{
		conn:     conn,
		sendCh:   make(chan frame),
		streams:  make(map[uint32]*Stream),
		nextID:   firstID,
		acceptCh: make(chan *Stream, acceptBacklog),
		done:     make(chan struct{}),
	}
	go s.readLoop()
	go s.writeLoop()
	return s
}

// OpenStream creates a new stream with the next free id
// and tells the peer with an open frame.
func (s *Session) OpenStream(label string) (*Stream, error) {
	if len(label) > maxPayload {
		return nil, ErrLabelTooLarge // it must fit in one frame
	}
	if s.isClosed() {
		return nil, ErrSessionClosed
	}
	s.mu.Lock()
	id := s.nextID
	s.nextID += 2
	st := newStream(id, s, label)
	s.streams[id] = st
	s.mu.Unlock()

	if err := s.send(frame{flags: flagOpen, id: id, payload: []byte(label)}); err != nil {
		s.removeStream(id)
		return nil, err
	}
	return st, nil
}

// Accept waits for the peer to open a new stream and returns it.
func (s *Session) Accept() (*Stream, error) {
	select {
	case st := <-s.acceptCh:
		return st, nil
	case <-s.done:
		return nil, ErrSessionClosed
	}
}

// Close ends the session and closes the underlying connection.
func (s *Session) Close() error {
	s.shutdown()
	return nil
}

func (s *Session) readLoop() {
	for {
		flags, id, payload, err := readFrame(s.conn)
		if err != nil {
			s.shutdown()
			return
		}

		s.mu.Lock()
		st, ok := s.streams[id]
		isNew := !ok && flags&flagOpen != 0
		if isNew {
			// the payload of an open frame is the label
			st = newStream(id, s, string(payload))
			s.streams[id] = st
		}
		s.mu.Unlock()

		if st == nil {
			continue // unknown id without open: drop it
		}

		data := payload
		if isNew {
			data = nil // it was the label, we don't pass that to stream Read

			select {
			case s.acceptCh <- st:
			case <-s.done:
				return
			}
		}

		if st.receive(data, flags&flagClose != 0) {
			s.removeStream(id) // both sides closed: forget it
		}
	}
}

func (s *Session) writeLoop() {
	for {
		select {
		case f := <-s.sendCh:
			if err := writeFrame(s.conn, f.flags, f.id, f.payload); err != nil {
				s.shutdown()
				return
			}
		case <-s.done:
			return
		}
	}
}

// send hands a frame to writeLoop, or fails if the session is dead.
func (s *Session) send(f frame) error {
	select {
	case s.sendCh <- f:
		return nil
	case <-s.done:
		return ErrSessionClosed
	}
}

func (s *Session) removeStream(id uint32) {
	s.mu.Lock()
	delete(s.streams, id)
	s.mu.Unlock()
}

// shutdown ends the session once: wakes everyone waiting on done
// and closes the conn. Safe to call many times.
func (s *Session) shutdown() {
	s.closeOnce.Do(func() {
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
