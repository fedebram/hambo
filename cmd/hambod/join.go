package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/fedebram/hambo/internal/api"
	"github.com/fedebram/hambo/internal/noiseconn"
	"github.com/fedebram/hambo/internal/rpc"
	"github.com/fedebram/hambo/internal/transport"
	"github.com/fedebram/hambo/internal/wireguard"
	"github.com/flynn/noise"
	"github.com/spf13/cobra"
)

// workerFile is everything a worker needs to connect to its server.
type workerFile struct {
	Server       string `json:"server"`
	ServerPublic []byte `json:"server_public"`
	Private      []byte `json:"private"`
	Public       []byte `json:"public"`
	WGPrivate    []byte `json:"wg_private"`
	WGPublic     []byte `json:"wg_public"`
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

// runJoin joins the server as a worker.
func runJoin(dataDir, server, token string) error {
	serverPublic, id, secret, err := parseJoinToken(token)
	if err != nil {
		return err
	}

	w, err := loadOrCreateWorker(dataDir, server, serverPublic)
	if err != nil {
		return err
	}
	key := noise.DHKey{Private: w.Private, Public: w.Public}

	sess, err := transport.Dial(w.Server, key, w.ServerPublic)
	if err != nil {
		return fmt.Errorf("can't connect to %s: %w", w.Server, err)
	}
	defer sess.Close()

	// "joined", or "already joined" if this worker joined before
	var answer string
	err = rpc.Call(sess, "join", api.JoinRequest{ID: id, Secret: secret, WGPublic: w.WGPublic}, &answer)
	var rpcErr *rpc.Error
	if errors.As(err, &rpcErr) {
		return fmt.Errorf("join failed: %w\ncreate a new token with: hambo token create", err)
	}
	if err != nil {
		return fmt.Errorf("join failed: %w\nrun hambod join again", err)
	}

	fmt.Println(answer, w.Server)
	return nil
}

func loadOrCreateWorker(dataDir, server string, serverPublic []byte) (workerFile, error) {
	path := filepath.Join(dataDir, "worker.json")

	var w workerFile
	data, err := os.ReadFile(path)
	switch {
	case err == nil:
		if err := json.Unmarshal(data, &w); err != nil {
			return workerFile{}, fmt.Errorf("%s: %w", path, err)
		}
		// a reset command?
		if !bytes.Equal(w.ServerPublic, serverPublic) {
			return workerFile{}, fmt.Errorf("this worker already joined another server (%s)\n"+
				"to join a different server, delete %s first", path, path)
		}
	case errors.Is(err, fs.ErrNotExist):
		if err := os.MkdirAll(dataDir, 0o700); err != nil {
			return workerFile{}, err
		}
		key, err := noiseconn.GenerateKey()
		if err != nil {
			return workerFile{}, err
		}
		w.Private = key.Private
		w.Public = key.Public
	default:
		return workerFile{}, err
	}

	if w.WGPrivate == nil {
		w.WGPrivate, w.WGPublic, err = wireguard.NewKey()
		if err != nil {
			return workerFile{}, err
		}
	}

	w.Server = server
	w.ServerPublic = serverPublic

	data, err = json.MarshalIndent(w, "", "  ")
	if err != nil {
		return workerFile{}, err
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return workerFile{}, err
	}
	return w, nil
}
