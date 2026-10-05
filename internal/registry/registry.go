package registry

import (
	_ "crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"

	"github.com/distribution/reference"
	ociv1 "github.com/opencontainers/image-spec/specs-go/v1"
)

const dockerMediaTypeManifestList = "application/vnd.docker.distribution.manifest.list.v2+json"
const dockerMediaTypeManifest = "application/vnd.docker.distribution.manifest.v2+json"

func FetchManifest(httpClient *http.Client, imageRef reference.Named) ([]byte, error) {
	registryHost := reference.Domain(imageRef)
	repository := reference.Path(imageRef)

	var ref string
	switch r := imageRef.(type) {
	case reference.Digested:
		ref = r.Digest().String()
	case reference.Tagged:
		ref = r.Tag()
	default:
		return nil, fmt.Errorf("image reference must include a tag or digest")
	}

	if registryHost == "docker.io" {
		registryHost = "registry-1.docker.io"
	}

	u := url.URL{
		Scheme: "https",
		Host:   registryHost,
		Path:   "/v2/" + repository + "/manifests/" + ref,
	}
	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	//github.com/opencontainers/image-spec/blob/v1.1.1/media-types.md#compatibility-matrix
	req.Header.Set("Accept",
		ociv1.MediaTypeImageIndex+","+
			ociv1.MediaTypeImageManifest+","+
			dockerMediaTypeManifestList+","+
			dockerMediaTypeManifest)

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch manifest: %s", resp.Status)
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		return nil, fmt.Errorf("manifest response is missing Content-Type")
	}

	// following oci distribution spec we ignore paramaters.
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil && err != mime.ErrInvalidMediaParameter {
		return nil, fmt.Errorf("invalid manifest Content-Type: %w", err)
	}

	switch mediaType {
	case ociv1.MediaTypeImageIndex,
		ociv1.MediaTypeImageManifest,
		dockerMediaTypeManifestList,
		dockerMediaTypeManifest:
	default:
		return nil, fmt.Errorf("unsupported manifest media type %q", mediaType)
	}

	manifest, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// oci distribution spec:
	// "..If the <tag-or-digest> part of a manifest request is a digest, clients SHOULD verify the returned manifest matches this digest."
	if r, ok := imageRef.(reference.Digested); ok {
		expected := r.Digest()
		actual := expected.Algorithm().FromBytes(manifest)

		if actual != expected {
			return nil, fmt.Errorf(
				"manifest digest mismatch: expected %s, got %s",
				expected, actual,
			)
		}
	}

	var documentMediaType string

	switch mediaType {
	case ociv1.MediaTypeImageManifest, dockerMediaTypeManifest:
		var parsed ociv1.Manifest
		if err := json.Unmarshal(manifest, &parsed); err != nil {
			return nil, fmt.Errorf("decoding image manifest: %w", err)
		}
		documentMediaType = parsed.MediaType

	case ociv1.MediaTypeImageIndex, dockerMediaTypeManifestList:
		var parsed ociv1.Index
		if err := json.Unmarshal(manifest, &parsed); err != nil {
			return nil, fmt.Errorf("decoding image index: %w", err)
		}
		documentMediaType = parsed.MediaType
	}

	// according to distribution oci spec we need to check the media type.
	if documentMediaType != "" && documentMediaType != mediaType {
		return nil, fmt.Errorf(
			"document mediaType %q does not match Content-Type %q",
			documentMediaType, mediaType,
		)
	}

	return manifest, nil
}

func ParseImageReference(value string) (reference.Named, error) {
	named, err := reference.ParseNamed(value)
	if err != nil {
		return nil, fmt.Errorf("parsing image reference %q: %w", value, err)
	}

	return reference.TagNameOnly(named), nil
}
