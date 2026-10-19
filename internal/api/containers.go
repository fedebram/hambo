package api

import (
	"errors"
	"net/http"
	"strings"
	"time"

	publicapi "github.com/fedebram/hambo/api"
	"github.com/fedebram/hambo/internal/container"
	bolt "go.etcd.io/bbolt"
)

func (srv *server) createContainerHandler(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 16*1024)

	var request publicapi.CreateContainerRequest
	if !srv.readJSON(w, r, &request) {
		return
	}

	request.Name = strings.TrimSpace(request.Name)
	request.Image = strings.TrimSpace(request.Image)

	fields := make(map[string]string)
	if request.Name == "" {
		fields["name"] = "must not be empty"
	}
	if request.Image == "" {
		fields["image"] = "must not be empty"
	}
	if len(fields) > 0 {
		srv.writeValidationErrorJSON(w, fields)
		return
	}

	id := container.GenerateContainerID()

	created := container.Container{
		ID:        id,
		Name:      request.Name,
		Image:     request.Image,
		State:     container.StatePending,
		CreatedAt: time.Now().UTC(),
	}

	errNameExists := errors.New("container name already exists")

	err := srv.db.WriteTx(func(tx *bolt.Tx) error {
		entries, err := srv.containers.ListTx(tx)
		if err != nil {
			return err
		}

		for _, entry := range entries {
			if entry.Value.Name == created.Name {
				return errNameExists
			}
		}

		return srv.containers.CreateTx(tx, created.ID, created)
	})
	if err != nil {
		if errors.Is(err, errNameExists) {
			srv.writeErrorJSON(
				w, http.StatusConflict,
				"container name already exists",
			)
			return
		}

		srv.logger.Error("store container", "error", err)
		srv.writeErrorJSON(
			w, http.StatusInternalServerError,
			"could not create container",
		)
		return
	}

	srv.schedulingQueue.Add(created.ID)

	srv.writeJSON(w, http.StatusCreated, publicapi.ContainerResponse{
		ID:                  created.ID,
		Name:                created.Name,
		Image:               created.Image,
		NodeID:              created.NodeID,
		State:               publicapi.ContainerState(created.State),
		ExitCode:            created.ExitCode,
		Reason:              created.Reason,
		Message:             created.Message,
		CreatedAt:           created.CreatedAt,
		DeletionRequestedAt: created.DeletionRequestedAt,
	})
}
