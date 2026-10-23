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
		Status: ContainerStatus{
			State: StatePending,
		},
	}
	d.mu.Lock()
	d.containers[c.ID] = c
	d.mu.Unlock()

	d.queue.Add(c.ID)

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

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(c)
}
