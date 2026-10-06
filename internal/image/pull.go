package image

import (
	"fmt"
	"net/http"

	"github.com/containerd/platforms"
	"github.com/distribution/reference"
	"github.com/fedebram/hambo/internal/blobstore"
	"github.com/fedebram/hambo/internal/registry"
	ociv1 "github.com/opencontainers/image-spec/specs-go/v1"
)

func Pull(httpClient *http.Client, store *blobstore.Store, imageRef reference.Named) (ociv1.Descriptor, error) {
	result, err := registry.FetchManifest(httpClient, imageRef)
	if err != nil {
		return ociv1.Descriptor{}, err
	}
	if _, err := store.PutBytes(result.Content); err != nil {
		return ociv1.Descriptor{}, err
	}

	target := result.Descriptor

	if result.Index != nil {
		platform := platforms.DefaultSpec()
		matcher := platforms.OnlyStrict(platform)

		var selected *ociv1.Descriptor
		for _, candidate := range result.Index.Manifests {
			if candidate.Platform != nil &&
				matcher.Match(*candidate.Platform) {
				selected = &candidate
				break
			}
		}

		if selected == nil {
			return ociv1.Descriptor{}, fmt.Errorf(
				"image has no manifest for platform %s",
				platforms.Format(platform),
			)
		}

		manifestRef, err := reference.WithDigest(
			reference.TrimNamed(imageRef),
			selected.Digest,
		)
		if err != nil {
			return ociv1.Descriptor{}, err
		}

		result, err = registry.FetchManifest(httpClient, manifestRef)
		if err != nil {
			return ociv1.Descriptor{}, err
		}
		// TODO: checks must be placed outside the pull function. Maybe on the registry package.
		if result.Descriptor.Size != selected.Size {
			return ociv1.Descriptor{}, fmt.Errorf(
				"manifest size mismatch: expected %d, got %d",
				selected.Size, result.Descriptor.Size,
			)
		}
		if result.Descriptor.MediaType != selected.MediaType {
			return ociv1.Descriptor{}, fmt.Errorf(
				"manifest media type mismatch: expected %q, got %q",
				selected.MediaType, result.Descriptor.MediaType,
			)
		}

		if _, err := store.PutBytes(result.Content); err != nil {
			return ociv1.Descriptor{}, err
		}
	}

	if result.Manifest == nil {
		return ociv1.Descriptor{}, fmt.Errorf("expected an image manifest")
	}

	return target, nil
}
