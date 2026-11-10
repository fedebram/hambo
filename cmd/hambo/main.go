package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"os"

	"github.com/fedebram/hambo/internal/noiseconn"
	"github.com/fedebram/hambo/muxer"
	"github.com/flynn/noise"
	"github.com/spf13/cobra"
)

func main() {
	var configPath string

	root := &cobra.Command{
		Use:          "hambo",
		Short:        "hambo controls a hambo server",
		SilenceUsage: true,
	}
	root.PersistentFlags().StringVar(&configPath, "config", "./data/admin.json", "the credentials file")

	root.AddCommand(pingCmd(&configPath))

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

type credentials struct {
	Server        string `json:"server"`
	ServerPublic  []byte `json:"server_public"`
	ClientPrivate []byte `json:"client_private"`
	ClientPublic  []byte `json:"client_public"`
}

// loadCredentials reads the credentials file at path.
func loadCredentials(path string) (credentials, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return credentials{}, fmt.Errorf("%s not found: run hambod init, or pass --config", path)
	}
	if err != nil {
		return credentials{}, err
	}

	var c credentials
	if err := json.Unmarshal(data, &c); err != nil {
		return credentials{}, fmt.Errorf("%s: %w", path, err)
	}
	return c, nil
}

// connect dials the server, runs the handshake and starts a session.
func connect(creds credentials) (*muxer.Session, error) {
	conn, err := net.Dial("tcp", creds.Server)
	if err != nil {
		return nil, err
	}

	key := noise.DHKey{Private: creds.ClientPrivate, Public: creds.ClientPublic}
	nc, err := noiseconn.Client(conn, key, creds.ServerPublic)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("handshake with %s failed (is this key allowed?): %w", creds.Server, err)
	}

	return muxer.Client(nc), nil
}
