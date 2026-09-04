package api

import (
	"context"
	"net/http"
	"strings"

	publicapi "github.com/fedebram/hambo/api"
	"github.com/fedebram/hambo/internal/image"
)

func (srv *server) pullImageHandler(w http.ResponseWriter, r *http.Request) {
	var input publicapi.PullImageRequest
	if !srv.readJSON(w, r, &input) {
		return
	}

	// TODO: reference parser and writejsonerror supporting validation err
	if strings.TrimSpace(input.Reference) == "" {
		srv.writeJSON(w, http.StatusUnprocessableEntity, publicapi.ErrorResponse{
			Code:    publicapi.ErrorCodeValidationFailed,
			Message: "request validation failed",
			Fields: map[string]string{
				"reference": "must be provided",
			},
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

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	writeEvent := func(event publicapi.ImagePullEvent) {
		if err := stream.Write(event); err != nil {
			cancel()
		}
	}

	result, err := srv.imageService.Pull(ctx, input.Reference, func(progress image.PullProgress) {
		writeEvent(publicapi.ImagePullEvent{
			Type:         "progress",
			Status:       progress.Event,
			Name:         progress.Name,
			Digest:       progress.Digest,
			CurrentBytes: progress.CurrentBytes,
			TotalBytes:   progress.TotalBytes,
		})
	})

	// TODO: refactor error handling

	if r.Context().Err() != nil {
		return
	}

	if err := stream.Err(); err != nil {
		srv.logger.Error("could not write image pull stream", "error", err)
		return
	}

	if err != nil {
		srv.logger.Error(err.Error(), "method", r.Method, "uri", r.URL.RequestURI())
		writeEvent(publicapi.ImagePullEvent{
			Type: "error",
			Error: publicapi.ErrorResponse{
				Code:    publicapi.ErrorCodeInternal,
				Message: "image pull failed",
			},
		})
		return
	}

	writeEvent(publicapi.ImagePullEvent{
		Type: "complete",
		Image: publicapi.Image{
			Reference: result.Reference,
			Digest:    result.Digest,
			SizeBytes: result.SizeBytes,
		},
	})
}

func (srv *server) listImagesHandler(w http.ResponseWriter, r *http.Request) {
	var filters []image.ListFilter
	// TODO: improve filtering
	if reference := r.URL.Query().Get("reference"); reference != "" {
		filters = append(filters, image.ByReference(reference))
	}

	images, err := srv.imageService.List(r.Context(), filters...)
	// TODO: better error handling. ctx err?
	if err != nil {
		srv.logger.Error(err.Error(), "method", r.Method, "uri", r.URL.RequestURI())
		srv.writeErrorJSON(
			w,
			http.StatusInternalServerError,
			publicapi.ErrorCodeInternal,
			"internal server error",
		)
		return
	}

	response := make([]publicapi.Image, 0, len(images))
	for _, image := range images {
		response = append(response, publicapi.Image{
			Reference: image.Reference,
			Digest:    image.Digest,
			SizeBytes: image.SizeBytes,
		})
	}

	srv.writeJSON(w, http.StatusOK, response)
}
