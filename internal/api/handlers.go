package api

import (
	"errors"
	"net/http"

	"github.com/fedebram/hambo/internal/container"
)

// TODO: return json error responses, including page not found and method not allowed (overriding the default mux behaviour)

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
		srv.writeJSON(w, http.StatusUnprocessableEntity, struct {
			Errors map[string]string `json:"errors"`
		}{
			Errors: problems,
		})
		return
	}

	c := container.Container{
		Name:  input.Name,
		Image: input.Image,
	}

	c, err := srv.service.Create(c)
	if errors.Is(err, container.ErrAlreadyExists) {
		http.Error(w, http.StatusText(http.StatusConflict), http.StatusConflict)
		return
	}
	if err != nil {
		srv.logger.Error(err.Error(), "method", r.Method, "uri", r.URL.RequestURI())
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	srv.writeJSON(w, http.StatusCreated, c)
}

func (srv *server) getContainerHandler(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")

	c, err := srv.service.Get(name)

	if errors.Is(err, container.ErrNotFound) {
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}
	if err != nil {
		srv.logger.Error(err.Error(), "method", r.Method, "uri", r.URL.RequestURI())
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	srv.writeJSON(w, http.StatusOK, c)
}

func (srv *server) startContainerHandler(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")

	c, err := srv.service.Start(name)
	if errors.Is(err, container.ErrNotFound) {
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}
	if errors.Is(err, container.ErrOperationNotAllowed) {
		http.Error(w, http.StatusText(http.StatusConflict), http.StatusConflict)
		return
	}
	if err != nil {
		srv.logger.Error(err.Error(), "method", r.Method, "uri", r.URL.RequestURI())
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	srv.writeJSON(w, http.StatusAccepted, c)
}

func (srv *server) stopContainerHandler(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")

	c, err := srv.service.Stop(name)
	if errors.Is(err, container.ErrNotFound) {
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}
	if errors.Is(err, container.ErrOperationNotAllowed) {
		http.Error(w, http.StatusText(http.StatusConflict), http.StatusConflict)
		return
	}
	if err != nil {
		srv.logger.Error(err.Error(), "method", r.Method, "uri", r.URL.RequestURI())
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	srv.writeJSON(w, http.StatusAccepted, c)
}

func (srv *server) deleteContainerHandler(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")

	c, err := srv.service.Delete(name)
	if errors.Is(err, container.ErrNotFound) {
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}
	if errors.Is(err, container.ErrOperationNotAllowed) {
		// TODO: a more informative response? On the container service we already enrich the error!
		http.Error(w, http.StatusText(http.StatusConflict), http.StatusConflict)
		return
	}
	if err != nil {
		srv.logger.Error(err.Error(), "method", r.Method, "uri", r.URL.RequestURI())
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	srv.writeJSON(w, http.StatusAccepted, c)
}
