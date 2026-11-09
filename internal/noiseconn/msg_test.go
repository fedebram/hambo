package noiseconn

import (
	"bytes"
	"errors"
	"io"
	"testing"

	"github.com/flynn/noise"
)

func TestMsgRoundTrip(t *testing.T) {
	var buf bytes.Buffer

	want := []byte("hello noise")
	if err := writeMsg(&buf, want); err != nil {
		t.Fatalf("write: %v", err)
	}

	got, err := readMsg(&buf)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestMsgTooLarge(t *testing.T) {
	var buf bytes.Buffer

	err := writeMsg(&buf, make([]byte, noise.MaxMsgLen+1))
	if !errors.Is(err, errMsgTooLarge) {
		t.Fatalf("got %v, want errMsgTooLarge", err)
	}
	if buf.Len() != 0 {
		t.Fatalf("nothing should be written, got %d bytes", buf.Len())
	}
}

func TestMsgTruncated(t *testing.T) {
	// length says 10 bytes, but only 3 follow
	buf := bytes.NewBuffer([]byte{0, 10, 'a', 'b', 'c'})

	_, err := readMsg(buf)
	if !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("got %v, want io.ErrUnexpectedEOF", err)
	}
}
