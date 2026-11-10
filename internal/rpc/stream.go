package rpc

import (
	"encoding/json"
	"errors"

	"github.com/fedebram/hambo/muxer"
)

type envelope struct {
	Data  any    `json:"data,omitempty"`
	Error string `json:"error,omitempty"`
}

// Stream sends and receives messages on one muxer stream.
type Stream struct {
	st  *muxer.Stream
	enc *json.Encoder
	dec *json.Decoder
}

func newStream(st *muxer.Stream) *Stream {
	return &Stream{
		st:  st,
		enc: json.NewEncoder(st),
		dec: json.NewDecoder(st),
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

// Close tells the other side we are done sending.
func (s *Stream) CloseSend() error {
	return s.st.Close()
}

// sendError sends an error message.
func (s *Stream) sendError(msg string) error {
	return s.enc.Encode(envelope{Error: msg})
}
