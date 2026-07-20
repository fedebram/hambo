package api

import (
	"net/http"
	"time"
)

type healthResponse struct {
	Status string `json:"status"`
}

type createContainerRequest struct {
	Name string `json:"name"`
}

type createContainerResponse struct {
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
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

	srv.writeJSON(w, http.StatusCreated, createContainerResponse{
		Name:      input.Name,
		CreatedAt: srv.now().UTC(),
	})
}
