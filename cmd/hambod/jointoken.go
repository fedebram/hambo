package main

import (
	"encoding/base64"
	"errors"
)

// joinToken puts the server's public key (32 bytes), the token ID (6 bytes)
// and the secret (32 bytes) one after the other, and encodes them as
// URL-safe base64 without padding.
func joinToken(serverPublic, id, secret []byte) string {
	var b []byte
	b = append(b, serverPublic...)
	b = append(b, id...)
	b = append(b, secret...)
	return base64.RawURLEncoding.EncodeToString(b)
}

// parseJoinToken reverses joinToken: it decodes s and splits it into
// the server's public key (32 bytes), the token ID (6) and the secret (32).
func parseJoinToken(s string) (serverPublic, id, secret []byte, err error) {
	b, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return nil, nil, nil, errors.New("invalid join token")
	}
	if len(b) != 32+6+32 {
		return nil, nil, nil, errors.New("invalid join token")
	}
	return b[0:32], b[32:38], b[38:70], nil
}
