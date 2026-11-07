package muxer

import (
	"encoding/binary"
	"errors"
	"io"
)

const (
	headerSize = 4
	maxPayload = 16 * 1024 // 16 KB
)

var ErrPayloadTooLarge = errors.New("muxer: payload too large")

func writeFrame(w io.Writer, payload []byte) error {
	if len(payload) > maxPayload {
		return ErrPayloadTooLarge
	}

	buf := make([]byte, headerSize+len(payload))
	binary.BigEndian.PutUint32(buf, uint32(len(payload)))
	copy(buf[headerSize:], payload)

	_, err := w.Write(buf)
	return err
}

func readFrame(r io.Reader) ([]byte, error) {
	header := make([]byte, headerSize)
	if _, err := io.ReadFull(r, header); err != nil {
		return nil, err
	}

	length := binary.BigEndian.Uint32(header)
	if length > maxPayload {
		return nil, ErrPayloadTooLarge
	}

	payload := make([]byte, length)
	if _, err := io.ReadFull(r, payload); err != nil {
		// second readfull might return EOF if the connection ended right after
		// reading the header. So we need to return unexpected eof
		if err == io.EOF {
			err = io.ErrUnexpectedEOF
		}
		return nil, err
	}

	return payload, nil
}
