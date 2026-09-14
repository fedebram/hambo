package image

import (
	"context"
	"fmt"

	"github.com/containerd/platforms"
	"github.com/distribution/reference"
)

func (s *Service) List(ctx context.Context) ([]Summary, error) {
	imageStore := s.client.ImageService()

	records, err := imageStore.List(
		ctx,
		fmt.Sprintf(`labels.%q`, ImageRecordKindLabel),
	)
	if err != nil {
		return nil, fmt.Errorf("listing images: %w", err)
	}

	publicRefDigests := make(map[string]struct{}, len(records))
	summaries := make([]Summary, 0, len(records))

	// creating public references
	for _, record := range records {
		if record.Labels[ImageRecordKindLabel] != ImageRecordKindReference {
			continue
		}
		publicDigest := record.Target.Digest.String()
		publicRefDigests[publicDigest] = struct{}{}

		var summary Summary
		ref, err := reference.ParseNamed(record.Name)
		if err != nil {
			return nil, err
		}
		switch ref := ref.(type) {
		// pull validation rejects references containing both a tag and a digest
		// so Hambo managed references are either tagged or canonical.
		case reference.NamedTagged:
			summary.Repository = ref.Name()
			summary.Tag = ref.Tag()
		case reference.Canonical:
			summary.Repository = ref.Name()
		default:
			return nil, fmt.Errorf(
				"stored image reference %q is neither tagged nor canonical",
				record.Name,
			)
		}

		summary.Digest = record.Target.Digest.String()
		summary.Size, err = record.Size(
			ctx,
			s.client.ContentStore(),
			platforms.Default(),
		)
		if err != nil {
			return nil, err
		}
		summaries = append(summaries, summary)
	}

	// creating dangling images = no public refs, internal ref present
	for _, record := range records {
		if record.Labels[ImageRecordKindLabel] != ImageRecordKindInternal {
			continue
		}
		// check if a public ref is already present
		_, ok := publicRefDigests[record.Target.Digest.String()]
		if ok {
			continue
		}

		var summary Summary
		summary.Digest = record.Target.Digest.String()
		summary.Size, err = record.Size(
			ctx,
			s.client.ContentStore(),
			platforms.Default(),
		)
		if err != nil {
			return nil, err
		}
		summaries = append(summaries, summary)
	}

	return summaries, nil
}
