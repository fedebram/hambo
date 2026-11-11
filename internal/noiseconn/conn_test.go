package noiseconn

import (
	"io"
	"net"
	"testing"
)

func TestConnRoundTrip(t *testing.T) {
	client, server := connPair(t)

	// client -> server
	go client.Write([]byte("hello"))

	buf := make([]byte, 5)
	if _, err := io.ReadFull(server, buf); err != nil {
		t.Fatalf("server read: %v", err)
	}
	if string(buf) != "hello" {
		t.Fatalf("server got %q", buf)
	}

	// server -> client
	go server.Write([]byte("world"))

	if _, err := io.ReadFull(client, buf); err != nil {
		t.Fatalf("client read: %v", err)
	}
	if string(buf) != "world" {
		t.Fatalf("client got %q", buf)
	}
}

// connPair returns a client and a server Conn that finished the handshake.
func connPair(t *testing.T) (*Conn, *Conn) {
	t.Helper()

	clientKey, err := GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	serverKey, err := GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	clientConn, serverConn := net.Pipe()
	t.Cleanup(func() {
		clientConn.Close()
		serverConn.Close()
	})
	allowAll := func([]byte) bool { return true }
	noTokens := func([]byte) []byte { return nil }

	type server struct {
		conn *Conn
		err  error
	}
	done := make(chan server, 1)
	go func() {
		c, _, err := Server(serverConn, serverKey, allowAll, noTokens)
		done <- server{c, err}
	}()

	client, err := Client(clientConn, clientKey, serverKey.Public)
	if err != nil {
		t.Fatalf("client: %v", err)
	}
	srv := <-done
	if srv.err != nil {
		t.Fatalf("server: %v", srv.err)
	}
	return client, srv.conn
}
