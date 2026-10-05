package registry

import (
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/distribution/reference"
	ociv1 "github.com/opencontainers/image-spec/specs-go/v1"
)

const dockerMediaTypeManifestList = "application/vnd.docker.distribution.manifest.list.v2+json"
const dockerMediaTypeManifest = "application/vnd.docker.distribution.manifest.v2+json"

func FetchManifest(httpClient *http.Client, registry, repository, ref string) ([]byte, error) {
	if registry == "docker.io" {
		registry = "registry-1.docker.io"
	}

	u := url.URL{
		Scheme: "https",
		Host:   registry,
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

	manifest, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return manifest, nil
}

func ParseImageReference(value string) (string, string, string, error) {
	named, err := reference.ParseNamed(value)
	if err != nil {
		return "", "", "", fmt.Errorf(
			"parsing image reference %q: %w", value, err,
		)
	}

	registryHost := reference.Domain(named)
	repository := reference.Path(named)

	var ref string
	switch r := reference.TagNameOnly(named).(type) {
	case reference.Digested:
		ref = r.Digest().String()
	case reference.Tagged:
		ref = r.Tag()
	}

	return registryHost, repository, ref, nil
}
