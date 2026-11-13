package noiseconn

import (
	"crypto/rand"
	"errors"
	"io"

	"github.com/flynn/noise"
)

var ErrNotConfirmed = errors.New("noiseconn: peer did not confirm the handshake")

// newHandshake creates the state for one handshake.
func newHandshake(initiator bool, local noise.DHKey, peerPub []byte) (*noise.HandshakeState, error) {
	return noise.NewHandshakeState(noise.Config{
		CipherSuite:   suite,
		Random:        rand.Reader,
		Pattern:       noise.HandshakeIK,
		Initiator:     initiator,
		Prologue:      []byte("hambo/v1"),
		StaticKeypair: local,
		PeerStatic:    peerPub,
	})
}

// clientHandshake runs IK as the initiator and returns the send and receive ciphers.
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

	// confirm: an empty encrypted message, to prove we have the session keys.
	// To block replayed msg1.
	confirm, err := cs1.Encrypt(nil, nil, nil)
	if err != nil {
		return nil, nil, err
	}
	if err := writeMsg(rw, confirm); err != nil {
		return nil, nil, err
	}

	return cs1, cs2, nil
}

// serverHandshake runs IK as the responder and also returns the client's public key.
func serverHandshake(rw io.ReadWriter, local noise.DHKey) (*noise.CipherState, *noise.CipherState, []byte, error) {
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

	peer := hs.PeerStatic()

	msg2, cs1, cs2, err := hs.WriteMessage(nil, nil)
	if err != nil {
		return nil, nil, nil, err
	}
	if err := writeMsg(rw, msg2); err != nil {
		return nil, nil, nil, err
	}

	// wait for the client's confirm
	// prevent replayed msg1
	confirm, err := readMsg(rw)
	if err != nil {
		return nil, nil, nil, err
	}
	if _, err := cs1.Decrypt(nil, nil, confirm); err != nil {
		return nil, nil, nil, ErrNotConfirmed
	}

	// swapped: the server sends on cs2, receives on cs1
	return cs2, cs1, peer, nil
}
