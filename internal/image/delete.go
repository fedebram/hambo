package image

import (
	"context"
	"fmt"
	"strings"

	"github.com/containerd/containerd/v2/core/images"
	containerderr "github.com/containerd/errdefs"
	"github.com/fedebram/hambo/errdefs"
	godigest "github.com/opencontainers/go-digest"
)

func (s *Service) deleteReference(ctx context.Context, refName string) (DeleteResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	normalizedRefName, err := NormalizeReference(refName)
	if err != nil {
		return DeleteResult{}, fmt.Errorf(
			"%w: invalid image reference %q: %v",
			errdefs.ErrInvalidArgument,
			refName,
			err,
		)
	}
	refName = normalizedRefName

	imageStore := s.client.ImageService()
	record, err := imageStore.Get(ctx, refName)
	if err != nil {
		if containerderr.IsNotFound(err) {
			return DeleteResult{}, fmt.Errorf(
				"get image reference %q: %w",
				refName,
				errdefs.ErrNotFound,
			)
		}
		return DeleteResult{}, fmt.Errorf(
			"get image reference %q: %w",
			refName,
			err,
		)
	}

	if record.Labels[ImageRecordKindLabel] != ImageRecordKindReference {
		return DeleteResult{}, fmt.Errorf(
			"image %q is not a Hambo managed public reference",
			refName,
		)
	}

	references, err := imageStore.List(
		ctx,
		fmt.Sprintf(
			`labels.%q==%q,target.digest==%q`,
			ImageRecordKindLabel,
			ImageRecordKindReference,
			record.Target.Digest.String(),
		),
	)
	if err != nil {
		return DeleteResult{}, fmt.Errorf(
			"listing references for image %q: %w",
			refName,
			err,
		)
	}

	// the image store list returns also the record!
	// So we need to skip and search for the other references.
	hasOtherReferences := false
	for _, other := range references {
		if other.Name != record.Name {
			hasOtherReferences = true
			break
		}
	}

	// if there are other references, we can safely delete the requested ref.
	// A container will show a stale public ref and a real digest, so, If needed, it is possible to inspect the underlying image
	// because the other refs prevent the image to be GC (of course i can use containerd leases but I don't know how to show things easily then)
	// This behaviour can be nice because when running containers we might want to inspect images used by these containers.
	// Since we rely on containerd and we don't store anything, then is good to prevent the image deletion.
	if hasOtherReferences {
		if err := imageStore.Delete(
			ctx,
			record.Name,
			images.DeleteTarget(&record.Target),
			images.SynchronousDelete(),
		); err != nil {
			return DeleteResult{}, fmt.Errorf(
				"deleting image reference %q: %w",
				record.Name,
				err,
			)
		}

		return DeleteResult{RemovedReference: record.Name}, nil
	}

	// we have only one public reference.
	// We search for containers that might use the image.
	// public policy: not possible to delete the image (last public ref or dangling image) if containers use it.

	imageDigest := record.Target.Digest.String()
	containers, err := s.client.ContainerService().List(
		ctx,
		fmt.Sprintf(
			`labels.%q==%q`,
			ContainerImageDigestLabel,
			imageDigest,
		),
	)
	if err != nil {
		return DeleteResult{}, fmt.Errorf(
			"listing containers using image %q: %w",
			imageDigest,
			err,
		)
	}
	// TODO: show all the containers that use this image?
	if len(containers) > 0 {
		return DeleteResult{}, fmt.Errorf(
			"%w: container %q uses image %q",
			errdefs.ErrOperationNotAllowed,
			containers[0].ID,
			imageDigest,
		)
	}

	// No containers use the image.
	// First try to delete the internal image. If not present (due to failing to creating one inside Pull)
	// Then proceeds to delete the public ref. We try first the internal because if we fail, then the public ref remains!
	if err := imageStore.Delete(
		ctx,
		internalImageName(record.Target),
		images.DeleteTarget(&record.Target),
		images.SynchronousDelete(),
	); err != nil && !containerderr.IsNotFound(err) {
		return DeleteResult{}, fmt.Errorf(
			"deleting internal image record for %q: %w",
			imageDigest,
			err,
		)
	}

	if err := imageStore.Delete(
		ctx,
		record.Name,
		images.DeleteTarget(&record.Target),
		images.SynchronousDelete(),
	); err != nil {
		return DeleteResult{}, fmt.Errorf(
			"deleting final image reference %q: %w",
			record.Name,
			err,
		)
	}

	return DeleteResult{
		RemovedReference: record.Name,
		RemovedImage:     imageDigest,
	}, nil
}

func (s *Service) deleteImage(ctx context.Context, digest string) (DeleteResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	imageStore := s.client.ImageService()
	requestedDigest := digest
	var records []images.Image

	// the user passed a digest with the alg part, so we suppose it is a full digest
	if strings.Contains(digest, ":") {
		d, err := godigest.Parse(digest)
		if err != nil {
			return DeleteResult{}, fmt.Errorf(
				"%w: parsing image digest %q: %v",
				errdefs.ErrInvalidArgument,
				digest,
				err,
			)
		}
		digest = d.String()

		records, err = imageStore.List(
			ctx,
			fmt.Sprintf(
				`labels.%q,target.digest==%q`,
				ImageRecordKindLabel,
				digest,
			),
		)
		if err != nil {
			return DeleteResult{}, fmt.Errorf(
				"listing records for image %q: %w",
				digest,
				err,
			)
		}
	} else {
		if !IsValidShortDigest(digest) {
			return DeleteResult{}, fmt.Errorf(
				"%w: invalid short image digest %q",
				errdefs.ErrInvalidArgument,
				digest,
			)
		}

		allRecords, err := imageStore.List(
			ctx,
			fmt.Sprintf(
				`labels.%q`,
				ImageRecordKindLabel,
			),
		)
		if err != nil {
			return DeleteResult{}, fmt.Errorf(
				"listing images while resolving short digest %q: %w",
				digest,
				err,
			)
		}

		matches := make(map[string]struct{})
		// it is possible that one shortened digests can be the prefix of more full digests.
		// even if it is very unlikely, we need to check that and refuse the deletion, maybe asking the user to provide a longer prefix or the full digest.
		for _, record := range allRecords {
			if strings.HasPrefix(record.Target.Digest.Encoded(), digest) {
				matches[record.Target.Digest.String()] = struct{}{}
			}
		}

		switch len(matches) {
		case 0:
			return DeleteResult{}, fmt.Errorf(
				"image with short digest %q: %w",
				digest,
				errdefs.ErrNotFound,
			)
		case 1:
			for match := range matches {
				digest = match
			}
		default:
			return DeleteResult{}, fmt.Errorf(
				"%w: short image digest %q is ambiguous",
				errdefs.ErrInvalidArgument,
				digest,
			)
		}

		for _, record := range allRecords {
			if record.Target.Digest.String() == digest {
				records = append(records, record)
			}
		}
	}
	if len(records) == 0 {
		return DeleteResult{}, fmt.Errorf(
			"image %q: %w",
			requestedDigest,
			errdefs.ErrNotFound,
		)
	}

	var reference images.Image
	var hasReference bool
	var internalRecord images.Image
	var hasInternalRecord bool
	for _, record := range records {
		switch record.Labels[ImageRecordKindLabel] {
		case ImageRecordKindReference:
			if hasReference {
				return DeleteResult{}, fmt.Errorf(
					"%w: image %q has multiple public references",
					errdefs.ErrOperationNotAllowed,
					digest,
				)
			}
			reference = record
			hasReference = true
		case ImageRecordKindInternal:
			if hasInternalRecord {
				return DeleteResult{}, fmt.Errorf(
					"image %q has multiple internal records",
					digest,
				)
			}
			internalRecord = record
			hasInternalRecord = true
		default:
			return DeleteResult{}, fmt.Errorf(
				"image record %q has an invalid Hambo record kind",
				record.Name,
			)
		}
	}

	containers, err := s.client.ContainerService().List(
		ctx,
		fmt.Sprintf(
			`labels.%q==%q`,
			ContainerImageDigestLabel,
			digest,
		),
	)
	if err != nil {
		return DeleteResult{}, fmt.Errorf(
			"listing containers using image %q: %w",
			digest,
			err,
		)
	}
	// TODO: list all the containers?
	if len(containers) > 0 {
		return DeleteResult{}, fmt.Errorf(
			"%w: container %q uses image %q",
			errdefs.ErrOperationNotAllowed,
			containers[0].ID,
			digest,
		)
	}

	if hasInternalRecord {
		if err := imageStore.Delete(
			ctx,
			internalRecord.Name,
			images.DeleteTarget(&internalRecord.Target),
			images.SynchronousDelete(),
		); err != nil {
			return DeleteResult{}, fmt.Errorf(
				"deleting internal image record %q: %w",
				internalRecord.Name,
				err,
			)
		}
	}

	var removedReference string
	if hasReference {
		if err := imageStore.Delete(
			ctx,
			reference.Name,
			images.DeleteTarget(&reference.Target),
			images.SynchronousDelete(),
		); err != nil {
			return DeleteResult{}, fmt.Errorf(
				"deleting image reference %q: %w",
				reference.Name,
				err,
			)
		}
		removedReference = reference.Name
	}

	return DeleteResult{
		RemovedReference: removedReference,
		RemovedImage:     digest,
	}, nil
}

func (s *Service) Delete(ctx context.Context, value string) (DeleteResult, error) {
	// since we don't support docker familiar names like "nginx". But we want normalized refs, then we can just check if there is /
	// to know that it is a delete by reference and not by digest.
	if strings.ContainsRune(value, '/') {
		return s.deleteReference(ctx, value)
	}

	return s.deleteImage(ctx, value)
}
