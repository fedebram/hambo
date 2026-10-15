package token

import (
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
)

func Parse(value string) (Token, error) {
	parts := strings.Split(value, ".")
	if len(parts) != 2 {
		return Token{}, errors.New("token must contain a CA fingerprint and secret")
	}

	for _, part := range parts {
		decoded, err := base64.RawURLEncoding.DecodeString(part)
		if err != nil {
			return Token{}, fmt.Errorf("decode token part: %w", err)
		}
		// ca fingerprint sha is 32 bytes. secret is also 32 bytes.
		// a mild check
		if len(decoded) != 32 {
			return Token{}, errors.New("each token part must contain 32 bytes")
		}
	}

	return Token{
		CAFingerprint: parts[0],
		Secret:        parts[1],
	}, nil
}
