package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"os"
	"path/filepath"

	"github.com/fedebram/hambo/internal/noiseconn"
	"github.com/fedebram/hambo/internal/rpc"
	"github.com/fedebram/hambo/muxer"
	"github.com/flynn/noise"
	"github.com/spf13/cobra"
)

var errUnreachable = errors.New("can't reach the server")

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

	// check if already joined with a normal connect
	err = connectAndPing(w.Server, func(conn net.Conn) (*noiseconn.Conn, error) {
		return noiseconn.Client(conn, key, w.ServerPublic)
	})
	if err == nil {
		fmt.Println("already joined", w.Server)
		return nil
	}
	if errors.Is(err, errUnreachable) {
		return err
	}

	// not joined
	err = connectAndPing(w.Server, func(conn net.Conn) (*noiseconn.Conn, error) {
		return noiseconn.Join(conn, key, w.ServerPublic, id, secret)
	})
	if err != nil {
		return fmt.Errorf("join failed: %w\n"+
			"run hambod join again; if it still fails, the token may be wrong, expired or already used:\n"+
			"create a new one with: hambo token create", err)
	}

	fmt.Println("joined", w.Server)
	return nil
}

// connectAndPing dials addr, runs the given handshake, and calls ping.
func connectAndPing(addr string, handshake func(net.Conn) (*noiseconn.Conn, error)) error {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return fmt.Errorf("%w: %v", errUnreachable, err)
	}
	nc, err := handshake(conn)
	if err != nil {
		conn.Close()
		return err
	}

	sess := muxer.Client(nc)
	defer sess.Close()

	s, err := rpc.NewStream(sess, "ping")
	if err != nil {
		return err
	}
	var answer string
	return s.Recv(&answer)
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
