package muxer

import (
	"bytes"
	"errors"
	"io"
	"net"
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

	st, err := client.OpenStream()
	if err != nil {
		t.Fatalf("OpenStream: %v", err)
	}
	if _, err := st.Write([]byte("hello")); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if err := st.CloseWrite(); err != nil {
		t.Fatalf("CloseWrite: %v", err)
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

	st, err := s.OpenStream()
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

// newPair returns a client and a server session connected to each othervia a net pipe.
func newPair(t *testing.T) (*Session, *Session) {
	clientConn, serverConn := net.Pipe()
	client := Client(clientConn)
	server := Server(serverConn)
	t.Cleanup(func() {
		client.Close()
		server.Close()
	})
	return client, server
}

// Close while the peer can still write: the peer gets a reset.
func TestCloseResetsPeer(t *testing.T) {
	client, server := newPair(t)

	st, err := client.OpenStream()
	if err != nil {
		t.Fatalf("OpenStream: %v", err)
	}
	peer, err := server.Accept()
	if err != nil {
		t.Fatalf("Accept: %v", err)
	}

	st.Close() // the server never sent FIN, so this sends a reset

	// Read waits until the reset arrives
	if _, err := peer.Read(make([]byte, 1)); !errors.Is(err, ErrStreamReset) {
		t.Fatalf("peer Read: got %v, want ErrStreamReset", err)
	}
	if _, err := peer.Write([]byte("x")); !errors.Is(err, ErrStreamReset) {
		t.Fatalf("peer Write: got %v, want ErrStreamReset", err)
	}
	// our side is closed for reading too
	if _, err := st.Read(make([]byte, 1)); !errors.Is(err, ErrStreamClosed) {
		t.Fatalf("own Read: got %v, want ErrStreamClosed", err)
	}
}

// Close after the peer sent FIN: the peer gets our data and a clean EOF.
func TestCloseAfterPeerFin(t *testing.T) {
	client, server := newPair(t)

	st, err := client.OpenStream()
	if err != nil {
		t.Fatalf("OpenStream: %v", err)
	}
	st.CloseWrite() // client: "I'm done writing"

	peer, err := server.Accept()
	if err != nil {
		t.Fatalf("Accept: %v", err)
	}
	// ReadAll returns only after the client's FIN arrived -> EOF
	if _, err := io.ReadAll(peer); err != nil {
		t.Fatalf("peer ReadAll: %v", err)
	}
	peer.Write([]byte("result"))
	peer.Close() // client already sent FIN, so this sends FIN, not reset

	got, err := io.ReadAll(st) // client side FIN arrived -> EOF
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if string(got) != "result" {
		t.Fatalf("got %q, want %q", got, "result")
	}
}

// Accept queue full: the next stream is refused with a reset.
func TestRefuseWhenAcceptQueueFull(t *testing.T) {
	client, _ := newPair(t) // nobody calls Accept on the server

	for i := 0; i < acceptBacklog; i++ {
		if _, err := client.OpenStream(); err != nil {
			t.Fatalf("OpenStream %d: %v", i, err)
		}
	}

	st, err := client.OpenStream() // one too many
	if err != nil {
		t.Fatalf("OpenStream: %v", err)
	}
	// Read waits until the reset arrives
	if _, err := st.Read(make([]byte, 1)); !errors.Is(err, ErrStreamReset) {
		t.Fatalf("Read: got %v, want ErrStreamReset", err)
	}
}

// Sending much more than the window only works if the reader grants more
// window as it reads, and the writer waits for it.
func TestTransferLargerThanWindow(t *testing.T) {
	client, server := newPair(t)

	want := make([]byte, 4*initialWindow)
	for i := range want {
		want[i] = byte(i % 251) // a pattern, so missing or reordered bytes show up
	}

	got := make(chan []byte, 1)
	go func() {
		peer, err := server.Accept()
		if err != nil {
			got <- nil
			return
		}
		data, _ := io.ReadAll(peer)
		got <- data
	}()

	st, err := client.OpenStream()
	if err != nil {
		t.Fatalf("OpenStream: %v", err)
	}
	if _, err := st.Write(want); err != nil {
		t.Fatalf("Write: %v", err)
	}
	st.CloseWrite()

	if !bytes.Equal(<-got, want) {
		t.Fatal("received data differs from what was sent")
	}
}
