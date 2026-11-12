package muxer

import (
	"errors"
	"fmt"
	"io"
	"sync"
)

var ErrSessionClosed = errors.New("muxer: session closed")

const acceptBacklog = 64 // streams the peer opened, waiting for Accept

type Session struct {
	conn   io.ReadWriteCloser
	sendCh chan writeRequest

	mu      sync.Mutex
	streams map[uint32]*Stream
	nextID  uint32

	acceptCh chan *Stream

	done      chan struct{}
	err       error
	closeOnce sync.Once
}

// writeRequest is a frame waiting to be written
type writeRequest struct {
	flags   uint16
	id      uint32
	payload []byte
	errCh   chan error
}

// Client starts a session on the client side. It opens streams with odd ids.
func Client(conn io.ReadWriteCloser) *Session {
	return newSession(conn, 1)
}

// Server starts a session on the server side. It opens streams with even ids.
func Server(conn io.ReadWriteCloser) *Session {
	return newSession(conn, 2)
}

func newSession(conn io.ReadWriteCloser, firstID uint32) *Session {
	s := &Session{
		conn:     conn,
		streams:  make(map[uint32]*Stream),
		nextID:   firstID,
		acceptCh: make(chan *Stream, acceptBacklog),
		sendCh:   make(chan writeRequest),
		done:     make(chan struct{}),
	}
	go s.readLoop()
	go s.writeLoop()
	return s
}

// OpenStream creates a new stream and tells the peer with an open frame.
func (s *Session) OpenStream() (*Stream, error) {
	s.mu.Lock()
	id := s.nextID
	s.nextID += 2
	st := newStream(id, s)
	s.streams[id] = st
	s.mu.Unlock()

	if err := s.send(flagOpen, id, nil); err != nil {
		s.removeStream(id)
		return nil, err
	}
	return st, nil
}

// Accept waits for the peer to open a stream and returns it.
func (s *Session) Accept() (*Stream, error) {
	select {
	case st := <-s.acceptCh:
		return st, nil
	case <-s.done:
		return nil, s.err
	}
}

// Close ends the session and closes the conn.
func (s *Session) Close() error {
	s.closeWithErr(nil)
	return nil
}

// closeWithErr ends the session once, remembering why.
func (s *Session) closeWithErr(err error) {
	s.closeOnce.Do(func() {
		s.err = ErrSessionClosed
		if err != nil {
			s.err = fmt.Errorf("%w: %v", ErrSessionClosed, err)
		}
		close(s.done)
		s.conn.Close()
	})
}

// isClosed reports whether the session has ended, without waiting.
func (s *Session) isClosed() bool {
	select {
	case <-s.done:
		return true
	default:
		return false
	}
}

// send hands a frame to writeLoop and waits until it is written.
func (s *Session) send(flags uint16, id uint32, payload []byte) error {
	req := writeRequest{flags, id, payload, make(chan error, 1)}

	select {
	case s.sendCh <- req:
	case <-s.done:
		return s.err
	}
	return <-req.errCh
}

func (s *Session) writeLoop() {
	for {
		select {
		case req := <-s.sendCh:
			err := writeFrame(s.conn, req.flags, req.id, req.payload)
			if err != nil {
				s.closeWithErr(err)
				req.errCh <- s.err
				return
			}
			req.errCh <- nil
		case <-s.done:
			return
		}
	}
}

func (s *Session) readLoop() {
	for {
		flags, id, payload, err := readFrame(s.conn)
		if err != nil {
			s.closeWithErr(err)
			return
		}

		s.mu.Lock()
		st := s.streams[id]
		s.mu.Unlock()

		if st == nil && flags&flagOpen != 0 {
			st = s.newRemoteStream(id)
		}
		if st == nil {
			continue
		}
		if st.receive(payload, flags&flagClose != 0) {
			s.removeStream(id)
		}
	}
}

// newRemoteStream creates a stream the peer opened and queues it for Accept.
// It returns nil if the stream is refused.
func (s *Session) newRemoteStream(id uint32) *Stream {
	st := newStream(id, s)

	s.mu.Lock()
	ours := id%2 == s.nextID%2
	if !ours {
		s.streams[id] = st
	}
	s.mu.Unlock()
	if ours {
		return nil
	}

	select {
	case s.acceptCh <- st:
		return st
	default: // queue full: refuse
		s.removeStream(id)
		go s.send(flagClose, id, nil)
		return nil
	}
}

func (s *Session) removeStream(id uint32) {
	s.mu.Lock()
	delete(s.streams, id)
	s.mu.Unlock()
}
