package muxer

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"testing"
)

type testFrame struct {
	flags   uint16
	id      uint32
	payload []byte
}

func TestFrameRoundTrip(t *testing.T) {
	tests := []struct {
		name   string
		frames []testFrame
	}{
		{"single", []testFrame{{0, 1, []byte("hello")}}},
		{"multiple stay separate", []testFrame{{0, 1, []byte("hello")}, {0, 3, []byte("world")}}},
		{"empty", []testFrame{{0, 1, []byte{}}}},
		{"max size", []testFrame{{0, 1, make([]byte, maxPayload)}}},
		{"flags", []testFrame{{flagOpen, 1, nil}, {flagOpen | flagClose, 3, []byte("hi")}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer

			for _, f := range tt.frames {
				if err := writeFrame(&buf, f.flags, f.id, f.payload); err != nil {
					t.Fatalf("writeFrame: %v", err)
				}
			}

			for i, want := range tt.frames {
				flags, id, got, err := readFrame(&buf)
				if err != nil {
					t.Fatalf("readFrame %d: %v", i, err)
				}
				if flags != want.flags {
					t.Fatalf("frame %d: got flags %b, want %b", i, flags, want.flags)
				}
				if id != want.id {
					t.Fatalf("frame %d: got id %d, want %d", i, id, want.id)
				}
				if !bytes.Equal(got, want.payload) {
					t.Fatalf("frame %d: got %d bytes, want %d", i, len(got), len(want.payload))
				}
			}

			// after the last frame, reading return EOF
			if _, _, _, err := readFrame(&buf); !errors.Is(err, io.EOF) {
				t.Fatalf("after last frame: got %v, want io.EOF", err)
			}
		})
	}
}

func TestWriteFrameTooLarge(t *testing.T) {
	var buf bytes.Buffer

	err := writeFrame(&buf, 0, 1, make([]byte, maxPayload+1))
	if !errors.Is(err, errPayloadTooLarge) {
		t.Fatalf("got %v, want ErrPayloadTooLarge", err)
	}
	if buf.Len() != 0 {
		t.Fatalf("buffer has %d bytes, want 0", buf.Len())
	}
}

func TestReadFrameErrors(t *testing.T) {
	tests := []struct {
		name  string
		input []byte
		want  error
	}{
		{"no data", nil, io.EOF},
		{"cut header in flags", []byte{0}, io.ErrUnexpectedEOF},
		{"cut header in id", []byte{0, 0, 0, 0}, io.ErrUnexpectedEOF},
		{"cut header in length", []byte{0, 0, 0, 0, 0, 1, 0, 0}, io.ErrUnexpectedEOF},
		{"cut payload", append(frameHeader(0, 1, 10), "abc"...), io.ErrUnexpectedEOF},
		{"header only, no payload", frameHeader(0, 1, 10), io.ErrUnexpectedEOF},
		{"too large", frameHeader(0, 1, maxPayload+1), errPayloadTooLarge},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, _, err := readFrame(bytes.NewReader(tt.input))
			if !errors.Is(err, tt.want) {
				t.Fatalf("got %v, want %v", err, tt.want)
			}
		})
	}
}

// frameHeader builds a raw header
func frameHeader(flags uint16, id, length uint32) []byte {
	h := make([]byte, headerSize)
	binary.BigEndian.PutUint16(h[0:2], flags)
	binary.BigEndian.PutUint32(h[2:6], id)
	binary.BigEndian.PutUint32(h[6:10], length)
	return h
}
