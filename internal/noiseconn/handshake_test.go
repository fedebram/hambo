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
	noTokens := func([]byte) []byte { return nil }

	type server struct {
		send, recv *noise.CipherState
		peer       Peer
		err        error
	}
	done := make(chan server, 1)
	go func() {
		send, recv, peer, err := serverHandshake(serverConn, serverKey, allowAll, noTokens)
		done <- server{send, recv, peer, err}
	}()

	send, recv, err := clientHandshake(clientConn, clientKey, serverKey.Public, nil, nil)
	if err != nil {
		t.Fatalf("client: %v", err)
	}

	srv := <-done
	if srv.err != nil {
		t.Fatalf("server: %v", srv.err)
	}

	if !bytes.Equal(srv.peer.Public, clientKey.Public) {
		t.Fatal("server got the wrong client key")
	}
	if srv.peer.TokenID != nil {
		t.Fatal("a normal connect should have no token")
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
	noTokens := func([]byte) []byte { return nil }

	done := make(chan error, 1)
	go func() {
		_, _, _, err := serverHandshake(serverConn, serverKey, denyAll, noTokens)
		if err != nil {
			// we drop the connection if the handshake fails
			serverConn.Close()
		}
		done <- err
	}()

	_, _, err = clientHandshake(clientConn, clientKey, serverKey.Public, nil, nil)
	if err == nil {
		t.Fatal("client should fail when the server rejects it")
	}

	// deny all -> the client is not allowed
	if err := <-done; !errors.Is(err, ErrNotAllowed) {
		t.Fatalf("server: got %v, want ErrNotAllowed", err)
	}
}

func TestHandshakeJoin(t *testing.T) {
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

	// the client is not on the allowlist: only its token can let it in
	denyAll := func([]byte) bool { return false }

	// the server knows one token
	tokenID := []byte{1, 2, 3, 4, 5, 6}
	secret := bytes.Repeat([]byte{7}, 32)
	joinSecret := func(id []byte) []byte {
		if bytes.Equal(id, tokenID) {
			return secret
		}
		return nil
	}

	type server struct {
		peer Peer
		err  error
	}
	done := make(chan server, 1)
	go func() {
		_, _, peer, err := serverHandshake(serverConn, serverKey, denyAll, joinSecret)
		done <- server{peer, err}
	}()

	_, _, err = clientHandshake(clientConn, clientKey, serverKey.Public, tokenID, secret)
	if err != nil {
		t.Fatalf("client: %v", err)
	}

	srv := <-done
	if srv.err != nil {
		t.Fatalf("server: %v", srv.err)
	}
	// the server learned who joined, and with which token
	if !bytes.Equal(srv.peer.Public, clientKey.Public) {
		t.Fatal("server got the wrong client key")
	}
	if !bytes.Equal(srv.peer.TokenID, tokenID) {
		t.Fatalf("token id: got %v, want %v", srv.peer.TokenID, tokenID)
	}
}

func TestHandshakeJoinWrongSecret(t *testing.T) {
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

	// the server's secret for this token is different from the client's
	tokenID := []byte{1, 2, 3, 4, 5, 6}
	joinSecret := func(id []byte) []byte {
		return bytes.Repeat([]byte{7}, 32)
	}

	done := make(chan error, 1)
	go func() {
		_, _, _, err := serverHandshake(serverConn, serverKey, denyAll, joinSecret)
		if err != nil {
			serverConn.Close()
		}
		done <- err
	}()

	wrong := bytes.Repeat([]byte{9}, 32)
	_, _, err = clientHandshake(clientConn, clientKey, serverKey.Public, tokenID, wrong)
	if err == nil {
		t.Fatal("client should fail with the wrong secret")
	}
	if err := <-done; err == nil {
		t.Fatal("server should reject the wrong secret")
	}
}
