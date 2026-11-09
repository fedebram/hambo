package noiseconn

import (
	"crypto/rand"
	"errors"
	"io"

	"github.com/flynn/noise"
)

var prologue = []byte("hambo/v0")

var ErrNotAllowed = errors.New("noiseconn: peer key not allowed")

func newHandshake(initiator bool, local noise.DHKey, peerPub []byte) (*noise.HandshakeState, error) {
	return noise.NewHandshakeState(noise.Config{
		CipherSuite:   suite,
		Random:        rand.Reader,
		Pattern:       noise.HandshakeIK,
		Initiator:     initiator,
		Prologue:      prologue,
		StaticKeypair: local,
		PeerStatic:    peerPub,
	})
}

func clientHandshake(rw io.ReadWriter, local noise.DHKey, serverPub []byte) (*noise.CipherState, *noise.CipherState, error) {
	hs, err := newHandshake(true, local, serverPub)
	if err != nil {
		return nil, nil, err
	}
	msg1, _, _, err := hs.WriteMessage(nil, nil)
	if err != nil {
		return nil, nil, err
	}

	if err := writeMsg(rw, msg1); err != nil {
		return nil, nil, err
	}

	msg2, err := readMsg(rw)
	if err != nil {
		return nil, nil, err
	}

	_, cs1, cs2, err := hs.ReadMessage(nil, msg2)
	if err != nil {
		return nil, nil, err
	}

	return cs1, cs2, nil
}

func serverHandshake(rw io.ReadWriter, local noise.DHKey, allowed func(pub []byte) bool) (*noise.CipherState, *noise.CipherState, []byte, error) {
	hs, err := newHandshake(false, local, nil)
	if err != nil {
		return nil, nil, nil, err
	}

	msg1, err := readMsg(rw)
	if err != nil {
		return nil, nil, nil, err
	}
	if _, _, _, err := hs.ReadMessage(nil, msg1); err != nil {
		return nil, nil, nil, err
	}

	// now we know who the client is: check before answering
	peer := hs.PeerStatic()
	if !allowed(peer) {
		return nil, nil, nil, ErrNotAllowed
	}

	msg2, cs1, cs2, err := hs.WriteMessage(nil, nil)
	if err != nil {
		return nil, nil, nil, err
	}
	if err := writeMsg(rw, msg2); err != nil {
		return nil, nil, nil, err
	}

	// swapped: the server sends on cs2, receives on cs1
	return cs2, cs1, peer, nil
}
