package muxer

import (
	"bytes"
	"errors"
	"io"
	"sync"
)

var ErrStreamClosed = errors.New("muxer: stream closed")

type Stream struct {
	id      uint32
	session *Session

	mu           sync.Mutex
	buf          bytes.Buffer
	localClosed  bool
	remoteClosed bool
	notify       chan struct{}
}

func newStream(id uint32, s *Session) *Stream {
	return &Stream{
		id:      id,
		session: s,
		notify:  make(chan struct{}, 1),
	}
}

// Read reads data received on this stream. It waits if none is available.
//
// It returns io.EOF when the peer closed the stream and all its data was read,
// or ErrSessionClosed if the session died first (data may be incomplete).
func (st *Stream) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	for {
		sessionDead := st.session.isClosed()

		st.mu.Lock()
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
			return 0, ErrSessionClosed // broke before the peer finished
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
		closed := st.localClosed
		st.mu.Unlock()
		if closed {
			return n, ErrStreamClosed
		}

		chunk := p[:min(len(p), maxPayload)]

		// the user might reuse the bytes. we need to clone
		f := frame{id: st.id, payload: bytes.Clone(chunk)}
		if err := st.session.send(f); err != nil {
			return n, err
		}

		n += len(chunk)
		p = p[len(chunk):]
	}
	return n, nil
}

// Close tells the peer we are done writing.
// Read keeps working until the peer closes too. Closing twice is a no-op.
func (st *Stream) Close() error {
	st.mu.Lock()
	if st.localClosed {
		st.mu.Unlock()
		return nil
	}
	st.localClosed = true
	bothClosed := st.remoteClosed
	st.mu.Unlock()

	if err := st.session.send(frame{flags: flagClose, id: st.id}); err != nil {
		return err
	}
	if bothClosed {
		st.session.removeStream(st.id)
	}
	return nil
}

// receive is called by readLoop for every frame of this stream.
// It reports whether both sides are now closed.
func (st *Stream) receive(payload []byte, remoteClose bool) bool {
	st.mu.Lock()
	st.buf.Write(payload)
	if remoteClose {
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
