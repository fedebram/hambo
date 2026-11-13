package noiseconn

import (
	"bytes"
	"errors"
	"net"
	"testing"

	"github.com/flynn/noise"
)

func mustKey(t *testing.T) noise.DHKey {
	t.Helper()
	key, err := GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	return key
}

func TestHandshake(t *testing.T) {
	clientKey, serverKey := mustKey(t), mustKey(t)
	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()

	type server struct {
		send, recv *noise.CipherState
		peer       []byte
		err        error
	}
	done := make(chan server, 1)
	go func() {
		send, recv, peer, err := serverHandshake(serverConn, serverKey)
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

	// the server learned who the client is
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

// The client expects a different server key: like talking to a fake server.
func TestHandshakeWrongServerKey(t *testing.T) {
	clientKey, serverKey, otherKey := mustKey(t), mustKey(t), mustKey(t)
	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()

	go func() {
		serverHandshake(serverConn, serverKey)
		serverConn.Close() // a failed handshake drops the connection
	}()

	_, _, err := clientHandshake(clientConn, clientKey, otherKey.Public)
	if err == nil {
		t.Fatal("client should fail when the server's key is not the one it expects")
	}
}

// Someone replays a real client's first message. They can't send the
// confirm, because they don't have the client's session keys.
func TestHandshakeReplayedMsg1(t *testing.T) {
	clientKey, serverKey := mustKey(t), mustKey(t)

	// the first message a real client sends, as an attacker could record it
	hs, err := newHandshake(true, clientKey, serverKey.Public)
	if err != nil {
		t.Fatal(err)
	}
	msg1, _, _, err := hs.WriteMessage(nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	attackerConn, serverConn := net.Pipe()
	defer attackerConn.Close()
	defer serverConn.Close()

	done := make(chan error, 1)
	go func() {
		_, _, _, err := serverHandshake(serverConn, serverKey)
		done <- err
	}()

	// the attacker replays msg1, reads msg2, and has to make up a confirm
	if err := writeMsg(attackerConn, msg1); err != nil {
		t.Fatal(err)
	}
	if _, err := readMsg(attackerConn); err != nil {
		t.Fatal(err)
	}
	if err := writeMsg(attackerConn, make([]byte, tagLen)); err != nil {
		t.Fatal(err)
	}

	if err := <-done; !errors.Is(err, ErrNotConfirmed) {
		t.Fatalf("server: got %v, want ErrNotConfirmed", err)
	}
}
