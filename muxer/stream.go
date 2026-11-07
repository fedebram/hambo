package muxer

import (
	"bytes"
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

func (st *Stream) Write(p []byte) (int, error)
