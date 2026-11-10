package rpc

import (
	"io"
	"net"
	"testing"

	"github.com/fedebram/hambo/muxer"
)

// streamPair returns the two ends of one stream, over a muxer session.
func streamPair(t *testing.T) (*Stream, *Stream) {
	t.Helper()

	clientConn, serverConn := net.Pipe()
	client := muxer.Client(clientConn)
	server := muxer.Server(serverConn)
	t.Cleanup(func() {
		client.Close()
		server.Close()
	})

	a, err := client.OpenStream("")
	if err != nil {
		t.Fatal(err)
	}
	b, err := server.Accept()
	if err != nil {
		t.Fatal(err)
	}
	return newStream(a), newStream(b)
}

type msg struct {
	Text string
}

func TestStreamSendRecv(t *testing.T) {
	a, b := streamPair(t)

	if err := a.Send(msg{Text: "ping"}); err != nil {
		t.Fatal(err)
	}
	var got msg
	if err := b.Recv(&got); err != nil {
		t.Fatal(err)
	}
	if got.Text != "ping" {
		t.Fatalf("got %q, want %q", got.Text, "ping")
	}
}

func TestStreamError(t *testing.T) {
	a, b := streamPair(t)

	if err := a.sendError("node w9 not found"); err != nil {
		t.Fatal(err)
	}

	var got msg
	err := b.Recv(&got)
	if err == nil || err.Error() != "node w9 not found" {
		t.Fatalf("got %v, want the remote error", err)
	}
	// an error message carries no data: got stays empty
	if got.Text != "" {
		t.Fatalf("got data %q with an error", got.Text)
	}
}

func TestStreamCloseSend(t *testing.T) {
	a, b := streamPair(t)

	a.Send(msg{Text: "last"})
	if err := a.CloseSend(); err != nil {
		t.Fatal(err)
	}

	var got msg
	if err := b.Recv(&got); err != nil || got.Text != "last" {
		t.Fatalf("got %q, %v", got.Text, err)
	}
	// nothing more: the other side is done
	if err := b.Recv(&got); err != io.EOF {
		t.Fatalf("got %v, want io.EOF", err)
	}
}
