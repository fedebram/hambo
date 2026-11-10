package muxer

import (
	"errors"
	"io"
	"net"
	"strings"
	"testing"
)

func TestStreamRoundTrip(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	client := Client(clientConn)
	server := Server(serverConn)
	defer client.Close()
	defer server.Close()

	got := make(chan string, 1)
	go func() {
		st, err := server.Accept()
		if err != nil {
			got <- "accept error: " + err.Error()
			return
		}
		// we continue reading until the client closes its side of the stream
		data, err := io.ReadAll(st)
		if err != nil {
			got <- "read error: " + err.Error()
			return
		}
		got <- string(data)
	}()

	st, err := client.OpenStream("")
	if err != nil {
		t.Fatalf("OpenStream: %v", err)
	}
	if _, err := st.Write([]byte("hello")); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if err := st.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	if g := <-got; g != "hello" {
		t.Fatalf("got %q, want %q", g, "hello")
	}
}

func TestStreams(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	client := Client(clientConn)
	server := Server(serverConn)
	defer client.Close()
	defer server.Close()

	// both sides accept one stream and echo it back
	go echo(server)
	go echo(client)

	// both sides open one stream
	sendAndCheck(t, client, "from client")
	sendAndCheck(t, server, "from server")
}

func TestStreamLabel(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	client := Client(clientConn)
	server := Server(serverConn)
	defer client.Close()
	defer server.Close()

	type accepted struct {
		label string
		data  string
		err   error
	}
	got := make(chan accepted, 1)
	go func() {
		st, err := server.Accept()
		if err != nil {
			got <- accepted{err: err}
			return
		}
		data, err := io.ReadAll(st)
		got <- accepted{st.Label(), string(data), err}
	}()

	st, err := client.OpenStream("nodes.list")
	if err != nil {
		t.Fatalf("OpenStream: %v", err)
	}
	// the opener sees its own label
	if st.Label() != "nodes.list" {
		t.Fatalf("opener label: got %q", st.Label())
	}
	st.Write([]byte("hello"))
	st.Close()

	a := <-got
	if a.err != nil {
		t.Fatalf("server: %v", a.err)
	}
	if a.label != "nodes.list" {
		t.Fatalf("server label: got %q, want %q", a.label, "nodes.list")
	}
	// the label must not show up as data
	if a.data != "hello" {
		t.Fatalf("server data: got %q, want %q", a.data, "hello")
	}
}

func TestStreamLabelTooLarge(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	client := Client(clientConn)
	server := Server(serverConn)
	defer client.Close()
	defer server.Close()

	_, err := client.OpenStream(strings.Repeat("x", maxPayload+1))
	if !errors.Is(err, ErrLabelTooLarge) {
		t.Fatalf("got %v, want ErrLabelTooLarge", err)
	}
}

// echo accepts one stream and sends back whatever it receives.
func echo(s *Session) {
	st, err := s.Accept()
	if err != nil {
		return
	}
	io.Copy(st, st)
}

// sendAndCheck opens a stream, writes msg and checks the echo comes back.
func sendAndCheck(t *testing.T, s *Session, msg string) {
	t.Helper()

	st, err := s.OpenStream("")
	if err != nil {
		t.Fatalf("OpenStream: %v", err)
	}
	if _, err := st.Write([]byte(msg)); err != nil {
		t.Fatalf("Write: %v", err)
	}

	buf := make([]byte, len(msg))
	if _, err := io.ReadFull(st, buf); err != nil {
		t.Fatalf("ReadFull: %v", err)
	}
	if string(buf) != msg {
		t.Fatalf("got %q, want %q", buf, msg)
	}
}
