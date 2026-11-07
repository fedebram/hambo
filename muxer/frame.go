package muxer

import (
	"encoding/binary"
	"errors"
	"io"
)

const (
	headerSize = 8         // streamID (4) + length (4)
	maxPayload = 16 * 1024 // 16 KB
)

var ErrPayloadTooLarge = errors.New("muxer: payload too large")

// writeFrame writes one frame: | streamID (4) | length (4) | payload |
func writeFrame(w io.Writer, id uint32, payload []byte) error {
	if len(payload) > maxPayload {
		return ErrPayloadTooLarge
	}

	buf := make([]byte, headerSize+len(payload))
	binary.BigEndian.PutUint32(buf[0:4], id)
	binary.BigEndian.PutUint32(buf[4:8], uint32(len(payload)))
	copy(buf[headerSize:], payload)

	_, err := w.Write(buf)
	return err
}

// readFrame reads one frame and returns its stream ID and payload.
func readFrame(r io.Reader) (uint32, []byte, error) {
	header := make([]byte, headerSize)
	if _, err := io.ReadFull(r, header); err != nil {
		return 0, nil, err
	}

	id := binary.BigEndian.Uint32(header[0:4])
	length := binary.BigEndian.Uint32(header[4:8])
	if length > maxPayload {
		return 0, nil, ErrPayloadTooLarge
	}

	payload := make([]byte, length)
	if _, err := io.ReadFull(r, payload); err != nil {
		// second readfull might return EOF if the connection ended right after
		// reading the header. So we need to return unexpected eof
		if err == io.EOF {
			err = io.ErrUnexpectedEOF
		}
		return 0, nil, err
	}

	return id, payload, nil
}
