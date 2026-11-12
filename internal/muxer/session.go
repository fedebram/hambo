package muxer

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"time"
)

var (
	ErrSessionClosed = errors.New("muxer: session closed")
	ErrGoAway        = errors.New("muxer: peer accepts no new streams")

	errKeepaliveTimeout = errors.New("keepalive timeout: nothing received from the peer")
)

const acceptBacklog = 64 // streams the peer opened, waiting for Accept

// Config holds the session settings. A zero field means its default,
// so Config{} gives all the defaults.
type Config struct {
	// KeepaliveInterval is how often we ping the peer. Default 10s.
	KeepaliveInterval time.Duration
	// KeepaliveTimeout closes the session when nothing at all has been
	// received from the peer for this long. Default 30s.
	KeepaliveTimeout time.Duration
}

type Session struct {
	conn   io.ReadWriteCloser
	sendCh chan writeRequest // data, open and FIN frames
	ctrlCh chan writeRequest // window updates and resets

	mu           sync.Mutex
	streams      map[uint32]*Stream
	nextID       uint32
	localGoAway  bool // we sent go away: refuse new streams from the peer
	remoteGoAway bool // the peer sent go away: OpenStream fails

	acceptCh chan *Stream

	cfg         Config
	lastRecv    atomic.Int64 // when we last received a frame (unix nanoseconds)
	pongPending atomic.Bool  // a pong is already on its way: don't start another

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
func Client(conn io.ReadWriteCloser, cfg Config) *Session {
	return newSession(conn, 1, cfg)
}

// Server starts a session on the server side. It opens streams with even ids.
func Server(conn io.ReadWriteCloser, cfg Config) *Session {
	return newSession(conn, 2, cfg)
}

func newSession(conn io.ReadWriteCloser, firstID uint32, cfg Config) *Session {
	if cfg.KeepaliveInterval == 0 {
		cfg.KeepaliveInterval = 10 * time.Second
	}
	if cfg.KeepaliveTimeout == 0 {
		cfg.KeepaliveTimeout = 30 * time.Second
	}
	s := &Session{
		conn:     conn,
		streams:  make(map[uint32]*Stream),
		nextID:   firstID,
		acceptCh: make(chan *Stream, acceptBacklog),
		sendCh:   make(chan writeRequest),
		ctrlCh:   make(chan writeRequest),
		done:     make(chan struct{}),

		cfg: cfg,
	}
	s.lastRecv.Store(time.Now().UnixNano())
	go s.readLoop()
	go s.writeLoop()
	go s.keepaliveLoop()
	return s
}

// OpenStream creates a new stream and tells the peer with an open frame.
// It fails with ErrGoAway if the peer said it accepts no new streams.
func (s *Session) OpenStream() (*Stream, error) {
	s.mu.Lock()
	if s.remoteGoAway {
		s.mu.Unlock()
		return nil, ErrGoAway
	}
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

// GoAway tells the peer we accept no new streams. Streams already open
// keep working; call Close when they are done. Calling it twice does nothing.
func (s *Session) GoAway() error {
	s.mu.Lock()
	if s.localGoAway {
		s.mu.Unlock()
		return nil
	}
	s.localGoAway = true
	s.mu.Unlock()

	return s.sendCtrl(flagGoAway, 0, nil)
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
	return s.sendOn(s.sendCh, flags, id, payload)
}

// sendCtrl is like send, for control frames: writeLoop writes them first.
func (s *Session) sendCtrl(flags uint16, id uint32, payload []byte) error {
	return s.sendOn(s.ctrlCh, flags, id, payload)
}

func (s *Session) sendOn(ch chan writeRequest, flags uint16, id uint32, payload []byte) error {
	req := writeRequest{flags, id, payload, make(chan error, 1)}

	select {
	case ch <- req:
	case <-s.done:
		return s.err
	}
	return <-req.errCh
}

// writeLoop is the only goroutine that writes to conn.
func (s *Session) writeLoop() {
	for {
		var req writeRequest
		// if there is something on the control channel, we first write that!
		select {
		case req = <-s.ctrlCh:
		default:
			select {
			case req = <-s.ctrlCh:
			case req = <-s.sendCh:
			case <-s.done:
				return
			}
		}

		err := writeFrame(s.conn, req.flags, req.id, req.payload)
		if err != nil {
			s.closeWithErr(err)
			req.errCh <- s.err
			return
		}
		req.errCh <- nil
	}
}

func (s *Session) readLoop() {
	for {
		flags, id, payload, err := readFrame(s.conn)
		if err != nil {
			s.closeWithErr(err)
			return
		}
		s.lastRecv.Store(time.Now().UnixNano()) // any frame shows the peer is alive

		if flags&flagPing != 0 {
			s.sendPong()
			continue
		}
		if flags&flagPong != 0 {
			continue // nothing more to do: lastRecv is already updated
		}

		if flags&flagGoAway != 0 {
			s.mu.Lock()
			s.remoteGoAway = true
			s.mu.Unlock()
			continue
		}

		s.mu.Lock()
		st := s.streams[id]
		s.mu.Unlock()

		if flags&flagReset != 0 {
			if st != nil {
				st.receiveReset()
				s.removeStream(id)
			}
			continue
		}
		if flags&flagWindow != 0 {
			if len(payload) != 4 {
				s.closeWithErr(errBadWindowFrame) // the peer broke the protocol
				return
			}
			if st != nil {
				st.receiveWindow(binary.BigEndian.Uint32(payload))
			}
			continue
		}
		if st == nil && flags&flagOpen != 0 {
			st = s.newRemoteStream(id)
		}
		if st == nil {
			continue
		}
		bothClosed, err := st.receive(payload, flags&flagFin != 0)
		if err != nil {
			s.closeWithErr(err) // the peer broke the protocol
			return
		}
		if bothClosed {
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
	goingAway := s.localGoAway
	if !ours && !goingAway {
		s.streams[id] = st
	}
	s.mu.Unlock()
	if ours {
		return nil
	}
	if goingAway { // we said no new streams
		go s.sendCtrl(flagReset, id, nil)
		return nil
	}

	select {
	case s.acceptCh <- st:
		return st
	default: // queue full: refuse
		s.removeStream(id)
		go s.sendCtrl(flagReset, id, nil)
		return nil
	}
}

// keepaliveLoop pings the peer regularly and closes the session
// when nothing has arrived for too long.
func (s *Session) keepaliveLoop() {
	ticker := time.NewTicker(s.cfg.KeepaliveInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
		case <-s.done:
			return
		}

		last := time.Unix(0, s.lastRecv.Load())
		if time.Since(last) > s.cfg.KeepaliveTimeout {
			s.closeWithErr(errKeepaliveTimeout)
			return
		}
		// not s.sendCtrl directly: if the peer is gone the write can get
		// stuck, and this loop must keep running to notice the timeout
		go s.sendCtrl(flagPing, 0, nil)
	}
}

// sendPong answers a ping without making readLoop wait.
// Only one pong is on its way at a time, so a peer flooding us with pings
// cannot make us start a goroutine for each one.
func (s *Session) sendPong() {
	if !s.pongPending.CompareAndSwap(false, true) {
		return // a pong is already on its way: that answers this ping too
	}
	go func() {
		s.sendCtrl(flagPong, 0, nil)
		s.pongPending.Store(false)
	}()
}

func (s *Session) removeStream(id uint32) {
	s.mu.Lock()
	delete(s.streams, id)
	s.mu.Unlock()
}
