package jointoken

import (
	"encoding/base64"
	"errors"
)

// the sizes of the three parts of a join token, in bytes
const (
	publicSize = 32
	idSize     = 6
	secretSize = 32
)

var errInvalid = errors.New("invalid join token")

// Encode puts the server's public key, the token ID
// and the secret one after the other, and encodes them as
// URL-safe base64 without padding.
func Encode(serverPublic, id, secret []byte) string {
	var b []byte
	b = append(b, serverPublic...)
	b = append(b, id...)
	b = append(b, secret...)
	return base64.RawURLEncoding.EncodeToString(b)
}

// parseJoinToken reverses Encode: it decodes s and splits it into
// the server's public key, the token ID and the secret.
func Parse(s string) (serverPublic, id, secret []byte, err error) {
	b, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return nil, nil, nil, errInvalid
	}
	if len(b) != publicSize+idSize+secretSize {
		return nil, nil, nil, errInvalid
	}
	serverPublic = b[:publicSize]
	id = b[publicSize : publicSize+idSize]
	secret = b[publicSize+idSize:]
	return serverPublic, id, secret, nil
}
