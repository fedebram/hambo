package muxer

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"testing"
)

func TestFrameRoundTrip(t *testing.T) {
	tests := []struct {
		name     string
		payloads [][]byte
	}{
		{"single", [][]byte{[]byte("hello")}},
		{"multiple stay separate", [][]byte{[]byte("hello"), []byte("world")}},
		{"empty", [][]byte{{}}},
		{"max size", [][]byte{make([]byte, maxPayload)}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer

			for _, p := range tt.payloads {
				if err := writeFrame(&buf, p); err != nil {
					t.Fatalf("writeFrame: %v", err)
				}
			}

			for i, want := range tt.payloads {
				got, err := readFrame(&buf)
				if err != nil {
					t.Fatalf("readFrame %d: %v", i, err)
				}
				if !bytes.Equal(got, want) {
					t.Fatalf("frame %d: got %d bytes, want %d", i, len(got), len(want))
				}
			}

			// after the last frame, reading return EOF
			if _, err := readFrame(&buf); !errors.Is(err, io.EOF) {
				t.Fatalf("after last frame: got %v, want io.EOF", err)
			}
		})
	}
}

func TestWriteFrameTooLarge(t *testing.T) {
	var buf bytes.Buffer

	err := writeFrame(&buf, make([]byte, maxPayload+1))
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
		{"cut header", []byte{0, 0}, io.ErrUnexpectedEOF},
		{"cut payload", append(frameHeader(10), "abc"...), io.ErrUnexpectedEOF},
		{"too large", frameHeader(maxPayload + 1), ErrPayloadTooLarge},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := readFrame(bytes.NewReader(tt.input))
			if !errors.Is(err, tt.want) {
				t.Fatalf("got %v, want %v", err, tt.want)
			}
		})
	}
}

// frameHeader builds a raw header
func frameHeader(length uint32) []byte {
	h := make([]byte, headerSize)
	binary.BigEndian.PutUint32(h, length)
	return h
}
