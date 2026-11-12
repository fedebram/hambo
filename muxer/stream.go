package muxer

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"sync"
)

var (
	ErrStreamClosed = errors.New("muxer: stream closed")
	ErrStreamReset  = errors.New("muxer: stream reset by peer")

	errWindowExceeded = errors.New("muxer: peer sent more than the window allows")
	errBadWindowFrame = errors.New("muxer: window frame payload is not 4 bytes")
)

// initialWindow is how many bytes each side may send on a stream
// before the reader catches up and grants more.
const initialWindow = 256 * 1024

// Stream is one bidirectional stream inside a session.
type Stream struct {
	id      uint32
	session *Session

	mu           sync.Mutex
	buf          bytes.Buffer
	localClosed  bool
	remoteClosed bool
	reset        bool
	closed       bool
	notify       chan struct{}

	sendWindow uint32        // bytes we may still send before the peer grants more
	recvWindow uint32        // bytes the peer may still send us
	sendNotify chan struct{}
}

func newStream(id uint32, s *Session) *Stream {
	return &Stream{
		id:         id,
		session:    s,
		notify:     make(chan struct{}, 1),
		sendWindow: initialWindow,
		recvWindow: initialWindow,
		sendNotify: make(chan struct{}, 1),
	}
}

// signal leaves a wake-up in ch without blocking.
// If one is already waiting, there is nothing to add.
func signal(ch chan struct{}) {
	select {
	case ch <- struct{}{}:
	default:
	}
}

// receive stores the payload of a frame and wakes up a waiting Read.
// It reports whether both sides are now closed, or an error if the peer
// sent more than its window allows.
func (st *Stream) receive(payload []byte, closed bool) (bool, error) {
	st.mu.Lock()
	if uint32(len(payload)) > st.recvWindow {
		st.mu.Unlock()
		return false, errWindowExceeded
	}
	st.recvWindow -= uint32(len(payload))
	st.buf.Write(payload)
	if closed {
		st.remoteClosed = true
	}
	bothClosed := st.localClosed && st.remoteClosed
	st.mu.Unlock()

	signal(st.notify)
	return bothClosed, nil
}

// receiveWindow is called by readLoop when the peer grants us more window.
func (st *Stream) receiveWindow(delta uint32) {
	st.mu.Lock()
	st.sendWindow += delta
	st.mu.Unlock()

	signal(st.sendNotify)
}

// receiveReset is called by readLoop when the peer abandons the stream.
func (st *Stream) receiveReset() {
	st.mu.Lock()
	st.reset = true
	st.buf.Reset()
	st.mu.Unlock()

	signal(st.notify)
	signal(st.sendNotify)
}

// Read reads data received on this stream. It waits if none is available.
// After reading, it grants the peer more window when enough space is free.
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
			grant := st.grantWindow()
			st.mu.Unlock()

			if grant > 0 {
				st.sendWindowUpdate(grant)
			}
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

// grantWindow works out how much new window to give the peer, and counts it
// as given. It returns 0 when it is not worth sending an update yet.
// st.mu must be held.
func (st *Stream) grantWindow() uint32 {
	if st.remoteClosed {
		return 0 // the peer sends nothing more
	}
	// space that is free in buf and not yet promised to the peer
	free := initialWindow - uint32(st.buf.Len()) - st.recvWindow
	if free < initialWindow/2 {
		return 0 // wait to free more
	}
	st.recvWindow += free
	return free
}

// sendWindowUpdate tells the peer it may send delta more bytes.
func (st *Stream) sendWindowUpdate(delta uint32) {
	var payload [4]byte
	binary.BigEndian.PutUint32(payload[:], delta)
	// if this fails the session is dead, and the next Read will say so
	st.session.sendCtrl(flagWindow, st.id, payload[:])
}

// Write sends p on this stream, split into frames of at most maxPayload bytes.
// It waits when the peer's window is used up, until the peer reads and grants more.
func (st *Stream) Write(p []byte) (int, error) {
	n := 0
	for len(p) > 0 {
		st.mu.Lock()
		if st.reset {
			st.mu.Unlock()
			return n, ErrStreamReset
		}
		if st.localClosed {
			st.mu.Unlock()
			return n, ErrStreamClosed
		}
		if st.sendWindow == 0 {
			st.mu.Unlock()
			select {
			case <-st.sendNotify:
			case <-st.session.done:
				return n, st.session.err
			}
			continue
		}
		size := min(len(p), maxPayload, int(st.sendWindow))
		st.sendWindow -= uint32(size)
		st.mu.Unlock()

		if err := st.session.send(0, st.id, p[:size]); err != nil {
			return n, err
		}

		n += size
		p = p[size:]
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

	signal(st.sendNotify) // a Write waiting for window must stop

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

	// wake up a Read or Write waiting in another goroutine
	signal(st.notify)
	signal(st.sendNotify)

	st.session.removeStream(st.id) // we ignore any frame that still arrives
	if peerWriting {
		return st.session.sendCtrl(flagReset, st.id, nil)
	}
	if needFin {
		return st.session.send(flagFin, st.id, nil)
	}
	return nil
}
