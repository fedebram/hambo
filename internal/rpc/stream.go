package rpc

import (
	"encoding/json"
	"errors"

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
		return errors.New(env.Error)
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
