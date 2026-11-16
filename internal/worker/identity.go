package worker

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/fedebram/hambo/internal/noiseconn"
	"github.com/fedebram/hambo/internal/wireguard"
	"github.com/flynn/noise"
)

// identity holds the node's keys and its server, saved in worker.json.
type identity struct {
	ServerAddress string `json:"server_address"`
	ServerPublic  []byte `json:"server_public"`
	Private       []byte `json:"private"`
	Public        []byte `json:"public"`
	WGPrivate     []byte `json:"wg_private"`
	WGPublic      []byte `json:"wg_public"`
}

// noiseKey returns the node's key for the handshake.
func (id identity) noiseKey() noise.DHKey {
	return noise.DHKey{Private: id.Private, Public: id.Public}
}

// loadIdentity reads worker.json from dataDir.
func loadIdentity(dataDir string) (identity, error) {
	path := filepath.Join(dataDir, "worker.json")
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return identity{}, fmt.Errorf("not joined: %s not found, run: hambod join --token <token>", path)
	}
	if err != nil {
		return identity{}, err
	}

	var id identity
	if err := json.Unmarshal(data, &id); err != nil {
		return identity{}, fmt.Errorf("%s: %w", path, err)
	}
	return id, nil
}

// loadOrCreateIdentity reads worker.json, or creates it with new keys.
// It fails if the node already belongs to another server.
func loadOrCreateIdentity(dataDir, address string, serverPublic []byte) (identity, error) {
	path := filepath.Join(dataDir, "worker.json")

	var id identity
	data, err := os.ReadFile(path)
	switch {
	case err == nil:
		if err := json.Unmarshal(data, &id); err != nil {
			return identity{}, fmt.Errorf("%s: %w", path, err)
		}
		if !bytes.Equal(id.ServerPublic, serverPublic) {
			return identity{}, fmt.Errorf("this worker already joined another server (%s)\n"+
				"to join a different server, delete %s first", path, path)
		}
	case errors.Is(err, fs.ErrNotExist):
		if err := os.MkdirAll(dataDir, 0o700); err != nil {
			return identity{}, err
		}
		key, err := noiseconn.GenerateKey()
		if err != nil {
			return identity{}, err
		}
		id.Private = key.Private
		id.Public = key.Public
	default:
		return identity{}, err
	}

	if id.WGPrivate == nil {
		id.WGPrivate, id.WGPublic, err = wireguard.NewKey()
		if err != nil {
			return identity{}, err
		}
	}

	id.ServerAddress = address
	id.ServerPublic = serverPublic

	data, err = json.MarshalIndent(id, "", "  ")
	if err != nil {
		return identity{}, err
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return identity{}, err
	}
	return id, nil
}
