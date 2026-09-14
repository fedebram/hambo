package image

import (
	// just to be sure
	_ "crypto/sha256"
	"fmt"
	"strings"

	"github.com/distribution/reference"
)

const ContainerImageDigestLabel = "io.hambo.container.image.digest"

type Summary struct {
	Repository string
	Tag        string
	Digest     string
	Size       int64
}

type DeleteResult struct {
	RemovedReference string
	RemovedImage     string
}

func NormalizeReference(value string) (string, error) {
	ref, err := reference.ParseNamed(value)
	if err != nil {
		return "", fmt.Errorf("parsing image reference %q: %w", value, err)
	}

	_, hasTag := ref.(reference.NamedTagged)
	_, hasDigest := ref.(reference.Digested)
	if hasTag && hasDigest {
		return "", fmt.Errorf(
			"parsing image reference %q: tag and digest cannot be combined",
			value,
		)
	}

	return reference.TagNameOnly(ref).String(), nil
}

// A valid short digest:
// - At least 12 chars
// - It is only the encoded part of the digest
// - It has only lowercase hex
func IsValidShortDigest(digest string) bool {
	if len(digest) < 12 {
		return false
	}
	for _, char := range digest {
		// the encoded part of the digest is lowercase hex
		if !strings.ContainsRune("0123456789abcdef", char) {
			return false
		}
	}

	return true
}
