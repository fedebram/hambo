package muxer

import (
	"bytes"
	"io"
	"sync"
)

type Stream struct {
	id      uint32
	session *Session

	mu     sync.Mutex
	buf    bytes.Buffer
	notify chan struct{}
}

func newStream(id uint32, s *Session) *Stream {
	return &Stream{
		id:      id,
		session: s,
		notify:  make(chan struct{}, 1),
	}
}

// Read reads data received on this stream. It waits if none is available.
func (st *Stream) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	for {
		closed := st.session.isClosed()

		st.mu.Lock()
		if st.buf.Len() > 0 {
			n, _ := st.buf.Read(p)
			st.mu.Unlock()
			return n, nil
		}
		st.mu.Unlock()

		if closed {
			// closed and the buffer is empty: return why the session ended
			// (io.EOF if the peer closed normally)
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
		chunk := p[:min(len(p), maxPayload)]

		// the user might reuse the bytes. we need to clone
		f := frame{id: st.id, payload: bytes.Clone(chunk)}

		select {
		case st.session.sendCh <- f:
		case <-st.session.done:
			err := st.session.err
			if err == io.EOF {
				// EOF comes from the read loop and means that the session closed
				err = ErrSessionClosed
			}
			return n, err
		}

		n += len(chunk)
		p = p[len(chunk):]
	}
	return n, nil
}
