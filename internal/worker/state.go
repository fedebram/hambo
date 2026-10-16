package worker

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

type state struct {
	ServerURL  string `json:"server_url"`
	Token      string `json:"token,omitempty"`
	Name       string `json:"name"`
	PrivateKey string `json:"private_key"`
	CSR        []byte `json:"csr,omitempty"`

	NodeID        string `json:"node_id,omitempty"`
	CACertificate string `json:"ca_certificate,omitempty"`
	Certificate   string `json:"certificate,omitempty"`
}

func loadState(dataDir string) (*state, error) {
	path := filepath.Join(dataDir, "worker.json")

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read worker state: %w", err)
	}

	var state state
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("decode worker state: %w", err)
	}

	return &state, nil
}

func saveState(dataDir string, state *state) (saveErr error) {
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("encode worker state: %w", err)
	}

	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return fmt.Errorf("create worker data directory: %w", err)
	}

	file, err := os.CreateTemp(dataDir, ".worker-*.json")
	if err != nil {
		return fmt.Errorf("create temporary worker state: %w", err)
	}
	defer file.Close()

	temporaryPath := file.Name()

	defer func() {
		err := os.Remove(temporaryPath)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			saveErr = errors.Join(
				saveErr,
				fmt.Errorf("remove temporary worker state: %w", err),
			)
		}
	}()

	if _, err := file.Write(data); err != nil {
		return fmt.Errorf("write worker state: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close worker state: %w", err)
	}

	path := filepath.Join(dataDir, "worker.json")
	if err := os.Rename(temporaryPath, path); err != nil {
		return fmt.Errorf("replace worker state: %w", err)
	}

	return nil
}
