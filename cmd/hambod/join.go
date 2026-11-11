package main

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/fedebram/hambo/internal/noiseconn"
	"github.com/spf13/cobra"
)

// workerFile is everything a worker needs to connect to its server.
type workerFile struct {
	Server       string `json:"server"`
	ServerPublic []byte `json:"server_public"`
	Private      []byte `json:"private"`
	Public       []byte `json:"public"`
}

func joinCmd() *cobra.Command {
	var dataDir string
	var server string
	var token string

	cmd := &cobra.Command{
		Use:   "join",
		Short: "Join a server as a worker, with a join token",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runJoin(dataDir, server, token)
		},
	}
	cmd.Flags().StringVar(&dataDir, "data-dir", "./data", "where to store the worker's files")
	cmd.Flags().StringVar(&server, "server", "127.0.0.1:6443", "the server's address")
	cmd.Flags().StringVar(&token, "token", "", "the join token, from: hambo token create")
	cmd.MarkFlagRequired("token")
	return cmd
}

func runJoin(dataDir, server, token string) error {
	serverPublic, id, _, err := parseJoinToken(token)
	if err != nil {
		return err
	}
	w, err := loadOrCreateWorker(dataDir, server, serverPublic)
	if err != nil {
		return err
	}
	fmt.Println("token id:", base64.RawURLEncoding.EncodeToString(id))
	fmt.Println("worker public key:", base64.StdEncoding.EncodeToString(w.Public))
	return nil
}

// loadOrCreateWorker returns the worker's saved identity, or creates
// and saves a new one.
func loadOrCreateWorker(dataDir, server string, serverPublic []byte) (workerFile, error) {
	path := filepath.Join(dataDir, "worker.json")

	data, err := os.ReadFile(path)
	if err == nil {
		var w workerFile
		if err := json.Unmarshal(data, &w); err != nil {
			return workerFile{}, fmt.Errorf("%s: %w", path, err)
		}
		return w, nil
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return workerFile{}, err
	}

	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return workerFile{}, err
	}
	key, err := noiseconn.GenerateKey()
	if err != nil {
		return workerFile{}, err
	}
	w := workerFile{
		Server:       server,
		ServerPublic: serverPublic,
		Private:      key.Private,
		Public:       key.Public,
	}
	data, err = json.MarshalIndent(w, "", "  ")
	if err != nil {
		return workerFile{}, err
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return workerFile{}, err
	}
	return w, nil
}
