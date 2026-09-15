package image

import (
	"context"
	"fmt"

	"github.com/containerd/containerd/v2/core/images"
	"github.com/containerd/containerd/v2/core/transfer"
	transferimage "github.com/containerd/containerd/v2/core/transfer/image"
	transferregistry "github.com/containerd/containerd/v2/core/transfer/registry"
	"github.com/containerd/errdefs"
	"github.com/containerd/platforms"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

const (
	internalImageRepository = "internal.hambo.io/image"

	ImageRecordKindLabel     = "io.hambo.image.record-kind"
	ImageRecordKindInternal  = "internal"
	ImageRecordKindReference = "reference"
)

type PullProgress struct {
	Event        string `json:"event"`
	Name         string `json:"name"`
	Digest       string `json:"digest,omitempty"`
	CurrentBytes int64  `json:"currentBytes"`
	TotalBytes   int64  `json:"totalBytes"`
}

type PullProgressFunc func(PullProgress)

// The user pull can pull an image in the form of:
// 	- registry/repo:tag
//  - registry/repo -> the latest tag gets appended because no tag is specified.
//  - registry/repo@sha
//
// It is not possible to pull an image with names like:
//  - registry/repo:tag@sha
//  - repo -> this will not be normalized like docker does.

// To preserve images already pulled, we create inside the containerd image store an internal record
// with the precise sha of the pulled image. This avoid the image to be GC by containerd, because otherwise the following might happens:
// - first pull of nginx:latest -> the underlying manifest sha is 123..
// - image store : nginx:latest = sha123
// - second pull of nginx:latest -> the underlying manifest sha is 456..
// - image store: nginx:latest = sha456
// - containerd GC the old image because there are no more references.
// This is a good behaviour but I'd like to implement something more docker like... with dangling images.
// When something like two pull of latest happens we have the internal image to protect GC and we can show on image listing the dangling image.
// This is nice also when taggging or moving tags. Basically we fake somehow an immutable internal image.

// During pulling might be possible that the internal image fails to be created. But it is not important. The important thing is that at least
// one image ref is present either public or internal.

// The user can think of images in terms of references (in fact containerd image records are somehow refs!). Reference to the underlying image.
// New references to the same image. When no more references are present pointing to the same image then there is a dangling image
// that internally is represented by the internal record we create.

// TODO: when pulling the transfer service creates a lease on the underyling content.
//       if we context cancel the pull, the lease over this content remains for 24 hours.
//       if we "resume" the pull, then on image listing we can see the resource and this is really nice.
//       But if we chose then to delete the image, the content remains because of the old lease that it is not get cleaned.
//       We need handle somehow leases... This behaviour doesn't happens on a successful pull because containerd delete the lease once the pull succeed.

func (s *Service) Pull(
	ctx context.Context,
	name string,
	report PullProgressFunc,
) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	normalizedName, err := NormalizeReference(name)
	if err != nil {
		return "", err
	}
	name = normalizedName

	source, err := transferregistry.NewOCIRegistry(ctx, name)
	if err != nil {
		return "", fmt.Errorf("creating registry source for image %q: %w", name, err)
	}

	platform := platforms.DefaultSpec()
	destination := transferimage.NewStore(
		name,
		transferimage.WithImageLabels(map[string]string{
			ImageRecordKindLabel: ImageRecordKindReference,
		}),
		transferimage.WithPlatforms(platform),
		transferimage.WithUnpack(platform, ""),
	)

	imageStore := s.client.ImageService()
	current, err := imageStore.Get(ctx, name)
	switch {
	case err == nil:
		if err := s.ensureInternalImage(ctx, current.Target); err != nil {
			return "", fmt.Errorf("protecting existing image %q before pull: %w", name, err)
		}
	case errdefs.IsNotFound(err):
		// This is the first pull of this reference.
	default:
		return "", fmt.Errorf("getting existing image %q before pull: %w", name, err)
	}

	progressFunc := func(progress transfer.Progress) {
		if report == nil {
			return
		}

		var digest string
		if progress.Desc != nil {
			digest = progress.Desc.Digest.String()
		}

		report(PullProgress{
			Event:        progress.Event,
			Name:         progress.Name,
			Digest:       digest,
			CurrentBytes: progress.Progress,
			TotalBytes:   progress.Total,
		})
	}

	if err := s.client.Transfer(
		ctx,
		source,
		destination,
		transfer.WithProgress(progressFunc),
	); err != nil {
		return "", fmt.Errorf("pulling image %q: %w", name, err)
	}

	record, err := imageStore.Get(ctx, name)
	if err != nil {
		return "", fmt.Errorf("getting pulled image %q: %w", name, err)
	}

	if err := s.ensureInternalImage(ctx, record.Target); err != nil {
		return "", fmt.Errorf("protecting pulled image %q: %w", name, err)
	}

	return record.Target.Digest.String(), nil
}

func (s *Service) ensureInternalImage(ctx context.Context, target ocispec.Descriptor) error {
	imageStore := s.client.ImageService()
	record := images.Image{
		Name:   internalImageName(target),
		Target: target,
		Labels: map[string]string{
			ImageRecordKindLabel: ImageRecordKindInternal,
		},
	}

	if _, err := imageStore.Create(ctx, record); err != nil && !errdefs.IsAlreadyExists(err) {
		return fmt.Errorf("creating internal record for target %q: %w", target.Digest, err)
	}
	return nil
}

func internalImageName(target ocispec.Descriptor) string {
	return internalImageRepository + "@" + target.Digest.String()
}
