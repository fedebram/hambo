package noiseconn

import (
	"bytes"
	"errors"
	"net"
	"testing"

	"github.com/flynn/noise"
)

func TestHandshake(t *testing.T) {
	clientKey, err := GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	serverKey, err := GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()
	allowAll := func([]byte) bool { return true }

	type server struct {
		send, recv *noise.CipherState
		peer       []byte
		err        error
	}
	done := make(chan server, 1)
	go func() {
		send, recv, peer, err := serverHandshake(serverConn, serverKey, allowAll)
		done <- server{send, recv, peer, err}
	}()

	send, recv, err := clientHandshake(clientConn, clientKey, serverKey.Public)
	if err != nil {
		t.Fatalf("client: %v", err)
	}

	srv := <-done
	if srv.err != nil {
		t.Fatalf("server: %v", srv.err)
	}

	if !bytes.Equal(srv.peer, clientKey.Public) {
		t.Fatal("server got the wrong client key")
	}

	// client -> server
	ct, err := send.Encrypt(nil, nil, []byte("ping"))
	if err != nil {
		t.Fatal(err)
	}
	pt, err := srv.recv.Decrypt(nil, nil, ct)
	if err != nil || string(pt) != "ping" {
		t.Fatalf("client -> server: got %q, %v", pt, err)
	}

	// server -> client
	ct, err = srv.send.Encrypt(nil, nil, []byte("pong"))
	if err != nil {
		t.Fatal(err)
	}
	pt, err = recv.Decrypt(nil, nil, ct)
	if err != nil || string(pt) != "pong" {
		t.Fatalf("server -> client: got %q, %v", pt, err)
	}
}

func TestHandshakeNotAllowed(t *testing.T) {
	clientKey, err := GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	serverKey, err := GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()
	denyAll := func([]byte) bool { return false }

	done := make(chan error, 1)
	go func() {
		_, _, _, err := serverHandshake(serverConn, serverKey, denyAll)
		if err != nil {
			// we drop the connection if the handshake fails
			serverConn.Close()
		}
		done <- err
	}()

	_, _, err = clientHandshake(clientConn, clientKey, serverKey.Public)
	if err == nil {
		t.Fatal("client should fail when the server rejects it")
	}

	// deny all -> the client is not allowed
	if err := <-done; !errors.Is(err, ErrNotAllowed) {
		t.Fatalf("server: got %v, want ErrNotAllowed", err)
	}
}
