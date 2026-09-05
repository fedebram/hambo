package image

import (
	"context"
	"fmt"

	"github.com/containerd/containerd/v2/client"
	containerdimages "github.com/containerd/containerd/v2/core/images"
	"github.com/containerd/containerd/v2/core/transfer"
	transferimage "github.com/containerd/containerd/v2/core/transfer/image"
	transferregistry "github.com/containerd/containerd/v2/core/transfer/registry"
	"github.com/containerd/errdefs"
	"github.com/containerd/platforms"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

type Service struct {
	client   *client.Client
	platform ocispec.Platform
}

func NewService(client *client.Client) *Service {
	return &Service{
		client:   client,
		platform: platforms.DefaultSpec(),
	}
}

func newRegistrySource(ctx context.Context, name string) (*transferregistry.OCIRegistry, error) {
	source, err := transferregistry.NewOCIRegistry(ctx, name)
	if err != nil {
		return nil, fmt.Errorf(
			"create registry source for image %q: %w",
			name,
			err,
		)
	}

	return source, nil
}

func (s *Service) newImageDestination(name string) *transferimage.Store {
	return transferimage.NewStore(
		name,
		transferimage.WithPlatforms(s.platform),
		transferimage.WithUnpack(s.platform, ""),
	)
}

// pull logic based on containerd ctr pull implementation. The transfer api is pretty cool.
// https://github.com/containerd/containerd/blob/main/cmd/ctr/commands/images/pull.go

func (s *Service) Pull(ctx context.Context, name string, report PullProgressFunc) (Image, error) {
	source, err := newRegistrySource(ctx, name)
	if err != nil {
		return Image{}, err
	}

	destination := s.newImageDestination(name)
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
		return Image{}, fmt.Errorf("pull image %q: %w", name, err)
	}

	containerdImage, err := s.client.GetImage(ctx, name)
	if err != nil {
		return Image{}, fmt.Errorf("get pulled image %q: %w", name, err)
	}

	sizeBytes, err := containerdImage.Size(ctx)
	if err != nil {
		return Image{}, fmt.Errorf("calculate size of pulled image %q: %w", name, err)
	}

	return Image{
		Name:      containerdImage.Name(),
		Digest:    containerdImage.Target().Digest.String(),
		SizeBytes: sizeBytes,
	}, nil
}

func (s *Service) List(ctx context.Context, filters ...ListFilter) ([]Image, error) {
	var config listFilters
	for _, filter := range filters {
		filter(&config)
	}

	// TODO: improve filtering

	containerdFilters := make([]string, 0, len(config.names))
	for _, name := range config.names {
		containerdFilters = append(
			containerdFilters,
			fmt.Sprintf("name==%q", name),
		)
	}

	containerdImages, err := s.client.ListImages(ctx, containerdFilters...)
	if err != nil {
		return nil, fmt.Errorf("list containerd images: %w", err)
	}

	result := make([]Image, 0, len(containerdImages))
	for _, containerdImage := range containerdImages {
		sizeBytes, err := containerdImage.Size(ctx)
		if err != nil {
			return nil, fmt.Errorf(
				"calculate size of image %q: %w",
				containerdImage.Name(),
				err,
			)
		}

		result = append(result, Image{
			Name:      containerdImage.Name(),
			Digest:    containerdImage.Target().Digest.String(),
			SizeBytes: sizeBytes,
		})
	}

	return result, nil
}

func (s *Service) Delete(ctx context.Context, name string) error {
	imageStore := s.client.ImageService()
	containerdImage, err := imageStore.Get(ctx, name)
	if err != nil {
		return fmt.Errorf("get image %q: %w", name, err)
	}

	if err := imageStore.Delete(
		ctx,
		name,
		containerdimages.DeleteTarget(&containerdImage.Target),
	); err != nil {
		if errdefs.IsNotFound(err) {
			return fmt.Errorf("image %q changed or was removed during deletion: %w", name, err)
		}
		return fmt.Errorf("delete image %q: %w", name, err)
	}

	return nil
}
