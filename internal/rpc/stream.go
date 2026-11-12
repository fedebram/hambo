package rpc

import (
	"encoding/json"
	"io"
	"time"

	"github.com/fedebram/hambo/internal/muxer"
)

type envelope struct {
	Data  any    `json:"data,omitempty"`
	Error string `json:"error,omitempty"`
}

// Stream sends and receives messages on one muxer stream.
type Stream struct {
	raw *muxer.Stream
	enc *json.Encoder
	dec *json.Decoder
}

func newStream(raw *muxer.Stream) *Stream {
	return &Stream{
		raw: raw,
		enc: json.NewEncoder(raw),
		dec: json.NewDecoder(raw),
	}
}

// Send sends v as one message.
func (s *Stream) Send(v any) error {
	return s.enc.Encode(envelope{Data: v})
}

// Recv reads one message into v.
func (s *Stream) Recv(v any) error {
	env := envelope{Data: v}
	if err := s.dec.Decode(&env); err != nil {
		return err
	}
	if env.Error != "" {
		return &Error{Msg: env.Error}
	}
	return nil
}

// CloseSend tells the other side we are done sending.
func (s *Stream) CloseSend() error {
	return s.raw.CloseWrite()
}

// Close ends the stream in both directions.
func (s *Stream) Close() error {
	return s.raw.Close()
}

// sendError sends an error message.
func (s *Stream) sendError(msg string) error {
	return s.enc.Encode(envelope{Error: msg})
}

// OpenStream opens a stream on the muxer session for the given method.
// The method name is sent as the first message, so the peer runs its handler.
func OpenStream(sess *muxer.Session, method string) (*Stream, error) {
	raw, err := sess.OpenStream()
	if err != nil {
		return nil, err
	}
	s := newStream(raw)
	if err := s.enc.Encode(method); err != nil {
		raw.Close()
		return nil, err
	}
	return s, nil
}

// finish ends a served call cleanly. We send FIN, then wait for the
// caller's FIN before closing: closing earlier would reset the stream,
// and the reset could throw away our answer before the caller reads it.
// If the caller's FIN doesn't come within lingerTimeout, we reset anyway.
func (s *Stream) finish() {
	s.CloseSend()

	timer := time.AfterFunc(lingerTimeout, func() { s.raw.Close() })
	io.Copy(io.Discard, s.raw) // returns at the caller's FIN, or at the reset
	timer.Stop()

	s.raw.Close() // both sides are done: this only frees the stream
}
