package noiseconn

import (
	"encoding/binary"
	"errors"
	"io"

	"github.com/flynn/noise"
)

// tagLen is the size of the authentication tag added by Encrypt.
// It is 16 for every cipher Noise allows (ChaChaPoly, AESGCM).
const tagLen = 16

// maxPlain is the most data that fits in one Noise message, tag included.
const maxPlain = noise.MaxMsgLen - tagLen

var errMsgTooLarge = errors.New("noiseconn: message too large")

// writeMsg writes one Noise message: | length (2) | message |
func writeMsg(w io.Writer, msg []byte) error {
	if len(msg) > noise.MaxMsgLen {
		return errMsgTooLarge
	}

	buf := make([]byte, 2+len(msg))
	binary.BigEndian.PutUint16(buf[0:2], uint16(len(msg)))
	copy(buf[2:], msg)

	_, err := w.Write(buf)
	return err
}

// readMsg reads one Noise message.
func readMsg(r io.Reader) ([]byte, error) {
	var header [2]byte
	if _, err := io.ReadFull(r, header[:]); err != nil {
		return nil, err
	}

	msg := make([]byte, binary.BigEndian.Uint16(header[:]))
	if _, err := io.ReadFull(r, msg); err != nil {
		// the connection ended after the length but before the message
		if err == io.EOF {
			err = io.ErrUnexpectedEOF
		}
		return nil, err
	}

	return msg, nil
}
