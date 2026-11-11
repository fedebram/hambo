package noiseconn

import (
	"errors"
	"io"
)

// mode says what kind of connection follows the prelude.
type mode byte

const (
	modeConnect mode = 1 // a known peer: plain IK
	modeJoin    mode = 2 // a new peer with a join token: IKpsk1
)

// tokenIDLen is the size of a join token's ID.
const tokenIDLen = 6

var errBadPrelude = errors.New("noiseconn: bad prelude")

// writePrelude writes the mode, and the token ID for a join.
func writePrelude(w io.Writer, m mode, tokenID []byte) error {
	b := []byte{byte(m)}
	if m == modeJoin {
		b = append(b, tokenID...)
	}
	_, err := w.Write(b)
	return err
}

// readPrelude reads the prelude.
// It also returns the raw bytes it read, for the prologue.
func readPrelude(r io.Reader) (m mode, tokenID []byte, raw []byte, err error) {
	b := make([]byte, 1)
	if _, err := io.ReadFull(r, b); err != nil {
		return 0, nil, nil, err
	}

	switch mode(b[0]) {
	case modeConnect:
		return modeConnect, nil, b, nil
	case modeJoin:
		id := make([]byte, tokenIDLen)
		if _, err := io.ReadFull(r, id); err != nil {
			return 0, nil, nil, err
		}
		return modeJoin, id, append(b, id...), nil
	default:
		return 0, nil, nil, errBadPrelude
	}
}
