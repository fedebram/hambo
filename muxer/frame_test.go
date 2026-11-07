package muxer

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"testing"
)

type testFrame struct {
	id      uint32
	payload []byte
}

func TestFrameRoundTrip(t *testing.T) {
	tests := []struct {
		name   string
		frames []testFrame
	}{
		{"single", []testFrame{{1, []byte("hello")}}},
		{"multiple stay separate", []testFrame{{1, []byte("hello")}, {3, []byte("world")}}},
		{"empty", []testFrame{{1, []byte{}}}},
		{"max size", []testFrame{{1, make([]byte, maxPayload)}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer

			for _, f := range tt.frames {
				if err := writeFrame(&buf, f.id, f.payload); err != nil {
					t.Fatalf("writeFrame: %v", err)
				}
			}

			for i, want := range tt.frames {
				id, got, err := readFrame(&buf)
				if err != nil {
					t.Fatalf("readFrame %d: %v", i, err)
				}
				if id != want.id {
					t.Fatalf("frame %d: got id %d, want %d", i, id, want.id)
				}
				if !bytes.Equal(got, want.payload) {
					t.Fatalf("frame %d: got %d bytes, want %d", i, len(got), len(want.payload))
				}
			}

			// after the last frame, reading return EOF
			if _, _, err := readFrame(&buf); !errors.Is(err, io.EOF) {
				t.Fatalf("after last frame: got %v, want io.EOF", err)
			}
		})
	}
}

func TestWriteFrameTooLarge(t *testing.T) {
	var buf bytes.Buffer

	err := writeFrame(&buf, 1, make([]byte, maxPayload+1))
	if !errors.Is(err, ErrPayloadTooLarge) {
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
		{"cut header in id", []byte{0, 0}, io.ErrUnexpectedEOF},
		{"cut header in length", []byte{0, 0, 0, 1, 0, 0}, io.ErrUnexpectedEOF},
		{"cut payload", append(frameHeader(1, 10), "abc"...), io.ErrUnexpectedEOF},
		{"too large", frameHeader(1, maxPayload+1), ErrPayloadTooLarge},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := readFrame(bytes.NewReader(tt.input))
			if !errors.Is(err, tt.want) {
				t.Fatalf("got %v, want %v", err, tt.want)
			}
		})
	}
}

// frameHeader builds a raw header
func frameHeader(id, length uint32) []byte {
	h := make([]byte, headerSize)
	binary.BigEndian.PutUint32(h[0:4], id)
	binary.BigEndian.PutUint32(h[4:8], length)
	return h
}
