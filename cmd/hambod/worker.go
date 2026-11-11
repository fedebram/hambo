package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/fedebram/hambo/internal/noiseconn"
	"github.com/fedebram/hambo/internal/rpc"
	"github.com/fedebram/hambo/muxer"
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
	r.Handle("health", func(s *rpc.Stream) error {
		return s.Send("ok")
	})

	for {
		err := connectAndServe(w, key, r)
		fmt.Println("disconnected:", err)
		fmt.Println("connecting again in 3s")
		time.Sleep(3 * time.Second)
	}
}

// connectAndServe connects to the server and serves its calls
// until the connection ends.
func connectAndServe(w workerFile, key noise.DHKey, r *rpc.Router) error {
	conn, err := net.Dial("tcp", w.Server)
	if err != nil {
		return err
	}
	nc, err := noiseconn.Client(conn, key, w.ServerPublic)
	if err != nil {
		conn.Close()
		return fmt.Errorf("handshake failed (has this worker joined?): %w", err)
	}

	sess := muxer.Client(nc)
	defer sess.Close()

	fmt.Println("connected to", w.Server)
	return r.Serve(sess)
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
