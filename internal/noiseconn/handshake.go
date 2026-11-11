package noiseconn

import (
	"crypto/rand"
	"errors"
	"io"

	"github.com/flynn/noise"
)

var (
	ErrNotAllowed   = errors.New("noiseconn: peer key not allowed")
	ErrNotConfirmed = errors.New("noiseconn: peer did not confirm the handshake")
	ErrUnknownToken = errors.New("noiseconn: unknown join token")
)

type Peer struct {
	Public  []byte
	TokenID []byte
}

// newHandshake creates the state for one handshake.
// The prelude is mixed into the prologue, so changing it breaks the handshake.
// With a psk, the pattern becomes IKpsk1, otherwise IK
func newHandshake(initiator bool, local noise.DHKey, peerPub, prelude, psk []byte) (*noise.HandshakeState, error) {
	p := append([]byte("hambo/v0"), prelude...)

	cfg := noise.Config{
		CipherSuite:   suite,
		Random:        rand.Reader,
		Pattern:       noise.HandshakeIK,
		Initiator:     initiator,
		Prologue:      p,
		StaticKeypair: local,
		PeerStatic:    peerPub,
	}
	if psk != nil {
		cfg.PresharedKey = psk
		cfg.PresharedKeyPlacement = 1 // psk1
	}
	return noise.NewHandshakeState(cfg)
}

func clientHandshake(rw io.ReadWriter, local noise.DHKey, serverPub, tokenID, psk []byte) (*noise.CipherState, *noise.CipherState, error) {
	m := modeConnect
	if tokenID != nil {
		m = modeJoin
	}

	prelude := []byte{byte(m)}
	if m == modeJoin {
		prelude = append(prelude, tokenID...)
	}
	if err := writePrelude(rw, m, tokenID); err != nil {
		return nil, nil, err
	}

	hs, err := newHandshake(true, local, serverPub, prelude, psk)
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

func serverHandshake(rw io.ReadWriter, local noise.DHKey, allowed func(pub []byte) bool, joinSecret func(id []byte) []byte) (*noise.CipherState, *noise.CipherState, Peer, error) {
	m, tokenID, prelude, err := readPrelude(rw)
	if err != nil {
		return nil, nil, Peer{}, err
	}

	// a join: the token's secret is the psk
	var psk []byte
	if m == modeJoin {
		psk = joinSecret(tokenID)
		if psk == nil {
			return nil, nil, Peer{}, ErrUnknownToken
		}
	}

	hs, err := newHandshake(false, local, nil, prelude, psk)
	if err != nil {
		return nil, nil, Peer{}, err
	}

	msg1, err := readMsg(rw)
	if err != nil {
		return nil, nil, Peer{}, err
	}
	// with a wrong psk, this fails
	if _, _, _, err := hs.ReadMessage(nil, msg1); err != nil {
		return nil, nil, Peer{}, err
	}

	peer := hs.PeerStatic()
	if m == modeConnect && !allowed(peer) {
		return nil, nil, Peer{}, ErrNotAllowed
	}

	msg2, cs1, cs2, err := hs.WriteMessage(nil, nil)
	if err != nil {
		return nil, nil, Peer{}, err
	}
	if err := writeMsg(rw, msg2); err != nil {
		return nil, nil, Peer{}, err
	}

	// wait for the client's confirm
	// prevent replayed msg1
	confirm, err := readMsg(rw)
	if err != nil {
		return nil, nil, Peer{}, err
	}
	if _, err := cs1.Decrypt(nil, nil, confirm); err != nil {
		return nil, nil, Peer{}, ErrNotConfirmed
	}

	// swapped: the server sends on cs2, receives on cs1
	return cs2, cs1, Peer{Public: peer, TokenID: tokenID}, nil
}
