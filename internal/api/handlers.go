package api

import (
	"errors"
	"fmt"
	"net/http"

	publicapi "github.com/fedebram/hambo/api"
	"github.com/fedebram/hambo/internal/container"
)

// TODO: return json error response for method not allowed and page not found (overriding the default mux)

type healthResponse struct {
	Status string `json:"status"`
}

type createContainerRequest struct {
	Name  string `json:"name"`
	Image string `json:"image"`
}

func (input createContainerRequest) valid() map[string]string {
	problems := make(map[string]string)

	if input.Name == "" {
		problems["name"] = "must be provided"
	}
	if input.Image == "" {
		problems["image"] = "must be provided"
	}

	return problems
}

func (srv *server) healthHandler(w http.ResponseWriter, r *http.Request) {
	srv.writeJSON(w, http.StatusOK, healthResponse{
		Status: "ok",
	})
}

func (srv *server) createContainerHandler(w http.ResponseWriter, r *http.Request) {
	var input createContainerRequest
	if !srv.readJSON(w, r, &input) {
		return
	}

	if problems := input.valid(); len(problems) > 0 {
		srv.writeJSON(w, http.StatusUnprocessableEntity, publicapi.ErrorResponse{
			Code:    publicapi.ErrorCodeValidationFailed,
			Message: "request validation failed",
			Fields:  problems,
		})
		return
	}

	c := container.Container{
		Name:  input.Name,
		Image: input.Image,
	}

	c, err := srv.service.Create(c)
	if errors.Is(err, container.ErrAlreadyExists) {
		srv.writeErrorJSON(
			w,
			http.StatusConflict,
			publicapi.ErrorCodeAlreadyExists,
			fmt.Sprintf("container %q already exists", input.Name),
		)
		return
	}
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
	srv.writeJSON(w, http.StatusCreated, c)
}

func (srv *server) getContainerHandler(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")

	c, err := srv.service.Get(name)

	if errors.Is(err, container.ErrNotFound) {
		srv.writeErrorJSON(
			w,
			http.StatusNotFound,
			publicapi.ErrorCodeNotFound,
			fmt.Sprintf("container %q not found", name),
		)
		return
	}
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

	srv.writeJSON(w, http.StatusOK, c)
}

func (srv *server) startContainerHandler(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")

	c, err := srv.service.Start(name)
	if errors.Is(err, container.ErrNotFound) {
		srv.writeErrorJSON(
			w,
			http.StatusNotFound,
			publicapi.ErrorCodeNotFound,
			fmt.Sprintf("container %q not found", name),
		)
		return
	}
	if errors.Is(err, container.ErrOperationNotAllowed) {
		srv.writeErrorJSON(
			w,
			http.StatusConflict,
			publicapi.ErrorCodeOperationNotAllowed,
			err.Error(),
		)
		return
	}
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

	srv.writeJSON(w, http.StatusAccepted, c)
}

func (srv *server) stopContainerHandler(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")

	c, err := srv.service.Stop(name)
	if errors.Is(err, container.ErrNotFound) {
		srv.writeErrorJSON(
			w,
			http.StatusNotFound,
			publicapi.ErrorCodeNotFound,
			fmt.Sprintf("container %q not found", name),
		)
		return
	}
	if errors.Is(err, container.ErrOperationNotAllowed) {
		srv.writeErrorJSON(
			w,
			http.StatusConflict,
			publicapi.ErrorCodeOperationNotAllowed,
			err.Error(),
		)
		return
	}
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

	srv.writeJSON(w, http.StatusAccepted, c)
}

func (srv *server) deleteContainerHandler(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")

	c, err := srv.service.Delete(name)
	if errors.Is(err, container.ErrNotFound) {
		srv.writeErrorJSON(
			w,
			http.StatusNotFound,
			publicapi.ErrorCodeNotFound,
			fmt.Sprintf("container %q not found", name),
		)
		return
	}
	if errors.Is(err, container.ErrOperationNotAllowed) {
		srv.writeErrorJSON(
			w,
			http.StatusConflict,
			publicapi.ErrorCodeOperationNotAllowed,
			err.Error(),
		)
		return
	}
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

	srv.writeJSON(w, http.StatusAccepted, c)
}
