package muxer

import (
	"encoding/binary"
	"errors"
	"io"
)

const (
	headerSize = 10        // flags (2) + streamID (4) + length (4)
	maxPayload = 16 * 1024 // 16 KB
)

const (
	flagOpen  uint16 = 1 << 0 // the sender opens a new stream
	flagFin   uint16 = 1 << 1 // the sender is done writing; it can still read
	flagReset uint16 = 1 << 2 // the sender abandoned the stream: no more reads or writes
)

var errPayloadTooLarge = errors.New("muxer: payload too large")

// writeFrame writes one frame: | flags (2) | streamID (4) | length (4) | payload |
func writeFrame(w io.Writer, flags uint16, id uint32, payload []byte) error {
	if len(payload) > maxPayload {
		return errPayloadTooLarge
	}

	buf := make([]byte, headerSize+len(payload))
	binary.BigEndian.PutUint16(buf[0:2], flags)
	binary.BigEndian.PutUint32(buf[2:6], id)
	binary.BigEndian.PutUint32(buf[6:10], uint32(len(payload)))
	copy(buf[headerSize:], payload)

	_, err := w.Write(buf)
	return err
}

// readFrame reads one frame and returns its flags, stream ID and payload.
func readFrame(r io.Reader) (uint16, uint32, []byte, error) {
	header := make([]byte, headerSize)
	if _, err := io.ReadFull(r, header); err != nil {
		return 0, 0, nil, err
	}

	flags := binary.BigEndian.Uint16(header[0:2])
	id := binary.BigEndian.Uint32(header[2:6])
	length := binary.BigEndian.Uint32(header[6:10])
	if length > maxPayload {
		return 0, 0, nil, errPayloadTooLarge
	}

	payload := make([]byte, length)
	if _, err := io.ReadFull(r, payload); err != nil {
		// second readfull might return EOF if the connection ended right after
		// reading the header. So we need to return unexpected eof
		if err == io.EOF {
			err = io.ErrUnexpectedEOF
		}
		return 0, 0, nil, err
	}

	return flags, id, payload, nil
}
