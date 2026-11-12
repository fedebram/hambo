package muxer

import (
	"bytes"
	"errors"
	"io"
	"sync"
)

var (
	ErrStreamClosed = errors.New("muxer: stream closed")
	ErrStreamReset  = errors.New("muxer: stream reset by peer")
)

// Stream is one bidirectional stream inside a session.
type Stream struct {
	id      uint32
	session *Session

	mu           sync.Mutex
	buf          bytes.Buffer
	localClosed  bool
	remoteClosed bool
	reset        bool
	closed       bool // we called Close: no more reads or writes
	notify       chan struct{}
}

func newStream(id uint32, s *Session) *Stream {
	return &Stream{
		id:      id,
		session: s,
		notify:  make(chan struct{}, 1),
	}
}

// receive stores the payload of a frame and wakes up a waiting Read.
// It reports whether both sides are now closed.
func (st *Stream) receive(payload []byte, closed bool) bool {
	st.mu.Lock()
	st.buf.Write(payload)
	if closed {
		st.remoteClosed = true
	}
	bothClosed := st.localClosed && st.remoteClosed
	st.mu.Unlock()

	select {
	case st.notify <- struct{}{}:
	default:
	}
	return bothClosed
}

// receiveReset is called by readLoop when the peer abandons the stream.
func (st *Stream) receiveReset() {
	st.mu.Lock()
	st.reset = true
	st.buf.Reset()
	st.mu.Unlock()

	select {
	case st.notify <- struct{}{}:
	default:
	}
}

// Read reads data received on this stream. It waits if none is available.
func (st *Stream) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	for {
		sessionDead := st.session.isClosed()

		st.mu.Lock()
		if st.closed {
			st.mu.Unlock()
			return 0, ErrStreamClosed
		}
		if st.reset {
			st.mu.Unlock()
			return 0, ErrStreamReset
		}
		if st.buf.Len() > 0 {
			n, _ := st.buf.Read(p)
			st.mu.Unlock()
			return n, nil
		}
		remoteClosed := st.remoteClosed
		st.mu.Unlock()

		if remoteClosed {
			return 0, io.EOF // peer finished: we have everything
		}
		if sessionDead {
			return 0, st.session.err
		}

		select {
		case <-st.notify:
		case <-st.session.done:
		}
	}
}

// Write sends p on this stream, split into frames of at most maxPayload bytes.
func (st *Stream) Write(p []byte) (int, error) {
	n := 0
	for len(p) > 0 {
		st.mu.Lock()
		reset := st.reset
		closed := st.localClosed
		st.mu.Unlock()
		if reset {
			return n, ErrStreamReset
		}
		if closed {
			return n, ErrStreamClosed
		}

		chunk := p[:min(len(p), maxPayload)]

		if err := st.session.send(0, st.id, chunk); err != nil {
			return n, err
		}

		n += len(chunk)
		p = p[len(chunk):]
	}
	return n, nil
}

// CloseWrite tells the peer we are done writing (a FIN).
// Read keeps working until the peer closes too. Calling it twice does nothing.
func (st *Stream) CloseWrite() error {
	st.mu.Lock()
	if st.localClosed {
		st.mu.Unlock()
		return nil
	}
	st.localClosed = true
	bothClosed := st.remoteClosed
	st.mu.Unlock()

	err := st.session.send(flagFin, st.id, nil)
	if bothClosed {
		st.session.removeStream(st.id)
	}
	return err
}

// Close ends the stream in both directions. Unread data is dropped.
// If the peer may still be writing, it sends a reset so the peer stops;
// otherwise a FIN, if we have not sent one yet. Closing twice does nothing.
func (st *Stream) Close() error {
	st.mu.Lock()
	if st.closed || st.reset {
		st.mu.Unlock()
		return nil
	}
	st.closed = true
	st.buf.Reset()
	peerWriting := !st.remoteClosed
	needFin := !st.localClosed
	st.localClosed = true
	st.mu.Unlock()

	// wake up a Read waiting in another goroutine
	select {
	case st.notify <- struct{}{}:
	default:
	}

	st.session.removeStream(st.id) // we ignore any frame that still arrives
	if peerWriting {
		return st.session.send(flagReset, st.id, nil)
	}
	if needFin {
		return st.session.send(flagFin, st.id, nil)
	}
	return nil
}
