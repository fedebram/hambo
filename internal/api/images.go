package api

import (
	"context"
	"errors"
	"net/http"
	"strings"

	publicapi "github.com/fedebram/hambo/api"
	"github.com/fedebram/hambo/errdefs"
	"github.com/fedebram/hambo/internal/image"
)

func (srv *server) pullImageHandler(w http.ResponseWriter, r *http.Request) {
	var input publicapi.PullImageRequest
	if !srv.readJSON(w, r, &input) {
		return
	}

	if strings.TrimSpace(input.Reference) == "" {
		srv.writeValidationErrorJSON(w, map[string]string{
			"reference": "must be provided",
		})
		return
	}

	// we need to check here because once we start the stream the http status code is 200.
	reference, err := image.NormalizeReference(input.Reference)
	if err != nil {
		srv.writeValidationErrorJSON(w, map[string]string{
			"reference": "must be a valid image reference",
		})
		return
	}

	stream, ok := startJSONStream(w, http.StatusOK)
	if !ok {
		srv.writeErrorJSON(
			w,
			http.StatusInternalServerError,
			publicapi.ErrorCodeInternal,
			"response streaming is not supported",
		)
		return
	}

	requestCtx := r.Context()
	pullCtx, cancelPull := context.WithCancel(requestCtx)
	defer cancelPull()

	digest, pullErr := srv.imageService.Pull(
		pullCtx,
		reference,
		func(progress image.PullProgress) {
			event := newImagePullProgressEvent(progress)
			if writeErr := stream.Write(event); writeErr != nil {
				cancelPull()
			}
		},
	)

	if writeErr := stream.Err(); writeErr != nil {
		if requestCtx.Err() == nil {
			srv.logger.Error(
				"could not write image pull stream",
				"error", writeErr,
				"reference", reference,
				"method", r.Method,
				"uri", r.URL.RequestURI(),
			)
		}
		return
	}

	if pullErr != nil {
		requestErr := requestCtx.Err()
		if requestErr != nil {
			if !errors.Is(pullErr, requestErr) {
				srv.logger.Error(
					pullErr.Error(),
					"reference", reference,
					"method", r.Method,
					"uri", r.URL.RequestURI(),
				)
			}
			return
		}

		if writeErr := stream.Write(
			newImagePullErrorEvent(reference, pullErr),
		); writeErr != nil {
			srv.logger.Error(
				"could not write image pull error event",
				"error", writeErr,
				"reference", reference,
				"method", r.Method,
				"uri", r.URL.RequestURI(),
			)
		}
		return
	}

	if requestCtx.Err() != nil {
		return
	}

	if writeErr := stream.Write(
		newImagePullCompletedEvent(reference, digest),
	); writeErr != nil && requestCtx.Err() == nil {
		srv.logger.Error(
			"could not write image pull completion event",
			"error", writeErr,
			"reference", reference,
			"method", r.Method,
			"uri", r.URL.RequestURI(),
		)
	}
}

func newImagePullProgressEvent(progress image.PullProgress) publicapi.ImagePullEvent {
	return publicapi.ImagePullEvent{
		Type:         "progress",
		Status:       progress.Event,
		Item:         progress.Name,
		Digest:       progress.Digest,
		CurrentBytes: progress.CurrentBytes,
		TotalBytes:   progress.TotalBytes,
	}
}

func newImagePullCompletedEvent(reference, digest string) publicapi.ImagePullEvent {
	return publicapi.ImagePullEvent{
		Type:      "completed",
		Reference: reference,
		Digest:    digest,
	}
}

func newImagePullErrorEvent(reference string, pullErr error) publicapi.ImagePullEvent {
	return publicapi.ImagePullEvent{
		Type:      "error",
		Reference: reference,
		Error: publicapi.ErrorResponse{
			Code:    publicapi.ErrorCodeImagePullFailed,
			Message: pullErr.Error(),
		},
	}
}

func (srv *server) listImagesHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	summaries, err := srv.imageService.List(ctx)
	if err != nil {
		requestErr := ctx.Err()
		if requestErr != nil && errors.Is(err, requestErr) {
			return
		}

		srv.logger.Error(err.Error(), "method", r.Method, "uri", r.URL.RequestURI())
		srv.writeErrorJSON(
			w,
			http.StatusInternalServerError,
			publicapi.ErrorCodeInternal,
			"internal server error",
		)
		return
	}

	srv.writeJSON(w, http.StatusOK, newListImagesResponse(summaries))
}

func newListImagesResponse(summaries []image.Summary) publicapi.ListImagesResponse {
	images := make([]publicapi.ImageSummary, 0, len(summaries))
	for _, summary := range summaries {
		images = append(images, publicapi.ImageSummary{
			Repository: summary.Repository,
			Tag:        summary.Tag,
			Digest:     summary.Digest,
			SizeBytes:  summary.Size,
		})
	}

	return publicapi.ListImagesResponse{Images: images}
}

func (srv *server) deleteImageHandler(w http.ResponseWriter, r *http.Request) {
	var input publicapi.DeleteImageRequest
	if !srv.readJSON(w, r, &input) {
		return
	}

	selector := strings.TrimSpace(input.Selector)
	if selector == "" {
		srv.writeValidationErrorJSON(w, map[string]string{
			"selector": "must be provided",
		})
		return
	}

	ctx := r.Context()
	result, err := srv.imageService.Delete(ctx, selector)
	if err != nil {
		requestErr := ctx.Err()
		if requestErr != nil && errors.Is(err, requestErr) {
			return
		}
		if errors.Is(err, errdefs.ErrInvalidArgument) {
			srv.writeValidationErrorJSON(w, map[string]string{
				"selector": "must be a valid and unambiguous image selector",
			})
			return
		}
		if errors.Is(err, errdefs.ErrNotFound) {
			srv.writeErrorJSON(
				w,
				http.StatusNotFound,
				publicapi.ErrorCodeNotFound,
				"image not found",
			)
			return
		}
		if errors.Is(err, errdefs.ErrOperationNotAllowed) {
			srv.writeErrorJSON(
				w,
				http.StatusConflict,
				publicapi.ErrorCodeOperationNotAllowed,
				err.Error(),
			)
			return
		}

		srv.logger.Error(err.Error(), "method", r.Method, "uri", r.URL.RequestURI())
		srv.writeErrorJSON(
			w,
			http.StatusInternalServerError,
			publicapi.ErrorCodeInternal,
			"internal server error",
		)
		return
	}

	srv.writeJSON(w, http.StatusOK, publicapi.DeleteImageResponse{
		RemovedReference: result.RemovedReference,
		RemovedImage:     result.RemovedImage,
	})
}
