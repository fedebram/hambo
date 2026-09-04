package image

import (
	"context"
	"fmt"

	"github.com/containerd/containerd/v2/client"
	"github.com/containerd/containerd/v2/core/transfer"
	transferimage "github.com/containerd/containerd/v2/core/transfer/image"
	transferregistry "github.com/containerd/containerd/v2/core/transfer/registry"
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

func newRegistrySource(ctx context.Context, reference string) (*transferregistry.OCIRegistry, error) {
	source, err := transferregistry.NewOCIRegistry(ctx, reference)
	if err != nil {
		return nil, fmt.Errorf(
			"create registry source for image %q: %w",
			reference,
			err,
		)
	}

	return source, nil
}

func (s *Service) newImageDestination(reference string) *transferimage.Store {
	return transferimage.NewStore(
		reference,
		transferimage.WithPlatforms(s.platform),
		transferimage.WithUnpack(s.platform, ""),
	)
}

// pull logic based on containerd ctr pull implementation. The transfer api is pretty cool.
// https://github.com/containerd/containerd/blob/main/cmd/ctr/commands/images/pull.go

func (s *Service) Pull(ctx context.Context, reference string, report PullProgressFunc) (Image, error) {
	source, err := newRegistrySource(ctx, reference)
	if err != nil {
		return Image{}, err
	}

	destination := s.newImageDestination(reference)
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
		return Image{}, fmt.Errorf("pull image %q: %w", reference, err)
	}

	containerdImage, err := s.client.GetImage(ctx, reference)
	if err != nil {
		return Image{}, fmt.Errorf("get pulled image %q: %w", reference, err)
	}

	sizeBytes, err := containerdImage.Size(ctx)
	if err != nil {
		return Image{}, fmt.Errorf("calculate size of pulled image %q: %w", reference, err)
	}

	return Image{
		Reference: containerdImage.Name(),
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

	containerdFilters := make([]string, 0, len(config.references))
	for _, reference := range config.references {
		containerdFilters = append(
			containerdFilters,
			fmt.Sprintf("name==%q", reference),
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
			Reference: containerdImage.Name(),
			Digest:    containerdImage.Target().Digest.String(),
			SizeBytes: sizeBytes,
		})
	}

	return result, nil
}
