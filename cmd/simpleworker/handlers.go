package main

import (
	"crypto/rand"
	"encoding/json"
	"io"
	"net/http"
)

func (d *daemon) handleRunContainer(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	var config ContainerConfig
	if err := decoder.Decode(&config); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		http.Error(w, "body must contain one JSON object", http.StatusBadRequest)
		return
	}

	applyContainerDefaults(&config)
	if err := validateContainer(config); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	c := Container{
		ID:              rand.Text(),
		ContainerConfig: config,
	}
	d.mu.Lock()
	d.containers[c.ID] = c
	d.mu.Unlock()

	// use the daemon context so creation continues after the response
	// here we simply spin up a goroutine per request. just to see it works.
	go func(c Container) {
		dockerID, err := runContainer(d.ctx, d.docker, c)

		d.mu.Lock()
		defer d.mu.Unlock()
		stored := d.containers[c.ID]
		stored.Status.DockerID = dockerID
		if err != nil {
			stored.Status.Error = err.Error()
		}
		d.containers[c.ID] = stored
	}(c)

	// pending is only for the acceptance response
	c.Status.State = StatePending
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Location", "/containers/"+c.ID)
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(c)
}

func (d *daemon) handleGetContainer(w http.ResponseWriter, r *http.Request) {
	d.mu.Lock()
	c, ok := d.containers[r.PathValue("id")]
	d.mu.Unlock()

	if !ok {
		http.Error(w, "container not found", http.StatusNotFound)
		return
	}

	if c.Status.DockerID == "" {
		c.Status.State = StatePending
		if c.Status.Error != "" {
			c.Status.State = StateFailed
		}
	} else {
		status, err := inspectContainer(r.Context(), d.docker, c.Status.DockerID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		c.Status = status
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(c)
}
