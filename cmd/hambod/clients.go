package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// role says what a client may do on the server.
type role string

const (
	roleUnknown role = ""
	roleAdmin   role = "admin"
	roleWorker  role = "worker"
)

// client is one known client.
type client struct {
	Name   string `json:"name"`
	Public []byte `json:"public"`
	Role   role   `json:"role"`
}

// clientsFile is what clients.json looks like.
type clientsFile struct {
	Clients []client `json:"clients"`
}

// clientStore holds the clients the server knows and their roles,
// loaded from clients.json and saved back to it.
type clientStore struct {
	mu      sync.Mutex
	clients map[string]client // public key → client
	path    string            // clients.json
}

// loadClients reads clients.json
func loadClients(dataDir string) (*clientStore, error) {
	cs := &clientStore{
		clients: make(map[string]client),
		path:    filepath.Join(dataDir, "clients.json"),
	}

	data, err := os.ReadFile(cs.path)
	if err != nil {
		return nil, err
	}

	var list clientsFile
	if err := json.Unmarshal(data, &list); err != nil {
		return nil, fmt.Errorf("%s: %w", cs.path, err)
	}
	for _, c := range list.Clients {
		cs.clients[string(c.Public)] = c
	}
	return cs, nil
}

// role returns the role of the client with this public key,
// or roleUnknown if the server doesn't know it.
func (cs *clientStore) role(pub []byte) role {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	c, ok := cs.clients[string(pub)]
	if !ok {
		return roleUnknown
	}
	return c.Role
}

// add puts a new client in the store and saves it.
func (cs *clientStore) add(c client) error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	cs.clients[string(c.Public)] = c
	if err := cs.save(); err != nil {
		delete(cs.clients, string(c.Public))
		return err
	}
	return nil
}

// save writes all clients to the file. The caller must hold cs.mu.
func (cs *clientStore) save() error {
	var list clientsFile
	for _, c := range cs.clients {
		list.Clients = append(list.Clients, c)
	}
	data, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(cs.path, data, 0o600)
}
