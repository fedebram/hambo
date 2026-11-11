package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// clientStore is the allowlist: the clients the server accepts,
// in memory and in clients.json.
type clientStore struct {
	mu    sync.Mutex
	names map[string]string // public key → name
	path  string            // clients.json
}

// loadClients reads clients.json
func loadClients(dataDir string) (*clientStore, error) {
	cs := &clientStore{
		names: make(map[string]string),
		path:  filepath.Join(dataDir, "clients.json"),
	}

	data, err := os.ReadFile(cs.path)
	if err != nil {
		return nil, err
	}

	var list allowlist
	if err := json.Unmarshal(data, &list); err != nil {
		return nil, fmt.Errorf("%s: %w", cs.path, err)
	}
	for _, c := range list.Clients {
		cs.names[string(c.Public)] = c.Name
	}
	return cs, nil
}

// allowed reports whether a client with this public key may connect.
func (cs *clientStore) allowed(pub []byte) bool {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	_, ok := cs.names[string(pub)]
	return ok
}

// add puts a new client in the allowlist and saves it.
func (cs *clientStore) add(name string, pub []byte) error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	cs.names[string(pub)] = name
	if err := cs.save(); err != nil {
		delete(cs.names, string(pub))
		return err
	}
	return nil
}

// save writes all clients to the file. The caller must hold cs.mu.
func (cs *clientStore) save() error {
	var list allowlist
	for pub, name := range cs.names {
		list.Clients = append(list.Clients, client{Name: name, Public: []byte(pub)})
	}
	data, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(cs.path, data, 0o600)
}
