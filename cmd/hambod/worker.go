package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/fedebram/hambo/internal/rpc"
	"github.com/fedebram/hambo/internal/transport"
	"github.com/flynn/noise"
	"github.com/spf13/cobra"
)

func workerCmd() *cobra.Command {
	var dataDir string

	cmd := &cobra.Command{
		Use:   "worker",
		Short: "Start the worker",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWorker(dataDir)
		},
	}
	cmd.Flags().StringVar(&dataDir, "data-dir", "./data", "where the worker's files are")
	return cmd
}

// runWorker connects to the server with the worker's saved identity
// and start its own router. If the connection drops, it connects again.
func runWorker(dataDir string) error {
	w, err := loadWorker(dataDir)
	if err != nil {
		return err
	}
	key := noise.DHKey{Private: w.Private, Public: w.Public}

	// the methods the server can call on this worker
	r := rpc.NewRouter()
	r.Handle("health", func(ctx context.Context, s *rpc.Stream) error {
		return s.Send("ok")
	})

	for {
		err := connectAndServe(w, key, r)
		if errors.Is(err, errNotJoined) {
			return err // retrying won't help
		}
		fmt.Println("disconnected:", err)
		fmt.Println("connecting again in 3s")
		time.Sleep(3 * time.Second)
	}
}

var errNotJoined = errors.New("not joined: run hambod join --token <token>")

// connectAndServe connects to the server and serves its calls
// until the connection ends.
func connectAndServe(w workerFile, key noise.DHKey, r *rpc.Router) error {
	sess, err := transport.Dial(w.Server, key, w.ServerPublic)
	if err != nil {
		return err
	}
	defer sess.Close()

	// the server refuses ping from a key it doesn't know
	var pong string
	err = rpc.Call(sess, "ping", nil, &pong)
	var rpcErr *rpc.Error
	if errors.As(err, &rpcErr) {
		return errNotJoined
	}
	if err != nil {
		return err
	}

	fmt.Println("connected to", w.Server)
	return r.Serve(context.Background(), sess)
}

// loadWorker reads the worker's identity from workers.json
func loadWorker(dataDir string) (workerFile, error) {
	path := filepath.Join(dataDir, "worker.json")
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return workerFile{}, fmt.Errorf("not joined: %s not found, run: hambod join --token <token>", path)
	}
	if err != nil {
		return workerFile{}, err
	}

	var w workerFile
	if err := json.Unmarshal(data, &w); err != nil {
		return workerFile{}, fmt.Errorf("%s: %w", path, err)
	}
	return w, nil
}
