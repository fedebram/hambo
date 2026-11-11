package main

import (
	"encoding/base64"
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

func serverCmd() *cobra.Command {
	var dataDir string
	var listen string

	cmd := &cobra.Command{
		Use:   "server",
		Short: "Start the server",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runServer(dataDir, listen)
		},
	}
	cmd.Flags().StringVar(&dataDir, "data-dir", "./data", "where the server's files are")
	cmd.Flags().StringVar(&listen, "listen", "127.0.0.1:6443", "the address to listen on")
	return cmd
}

func runServer(dataDir, listen string) error {
	key, err := loadServerKey(dataDir)
	if err != nil {
		return err
	}
	fmt.Println("server public key:", base64.StdEncoding.EncodeToString(key.Public))

	allowed, err := loadAllowlist(dataDir)
	if err != nil {
		return err
	}

	tokens, err := loadTokens(dataDir)
	if err != nil {
		return err
	}

	// the methods clients can call
	r := rpc.NewRouter()
	r.Handle("ping", func(s *rpc.Stream) error {
		return s.Send("pong")
	})
	r.Handle("tokens.create", func(s *rpc.Stream) error {
		id, secret, err := tokens.create()
		if err != nil {
			return err
		}
		return s.Send(joinToken(key.Public, id, secret))
	})

	ln, err := net.Listen("tcp", listen)
	if err != nil {
		return err
	}
	defer ln.Close()
	fmt.Println("listening on", ln.Addr())

	for {
		conn, err := ln.Accept()
		if err != nil {
			return err
		}
		go serveConn(conn, key, allowed, r)
	}
}

func serveConn(conn net.Conn, key noise.DHKey, allowed func([]byte) bool, r *rpc.Router) {
	nc, _, err := noiseconn.Server(conn, key, allowed)
	if err != nil {
		fmt.Println("handshake failed:", conn.RemoteAddr(), err)
		conn.Close()
		return
	}

	sess := muxer.Server(nc)
	defer sess.Close()

	r.Serve(sess)
}

func loadServerKey(dataDir string) (noise.DHKey, error) {
	path := filepath.Join(dataDir, "server.json")
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return noise.DHKey{}, fmt.Errorf("not initialized: %s not found, run: hambod init", path)
	}
	if err != nil {
		return noise.DHKey{}, err
	}

	var kf keyFile
	if err := json.Unmarshal(data, &kf); err != nil {
		return noise.DHKey{}, fmt.Errorf("%s: %w", path, err)
	}
	return noise.DHKey{Private: kf.Private, Public: kf.Public}, nil
}

func loadAllowlist(dataDir string) (func(pub []byte) bool, error) {
	path := filepath.Join(dataDir, "clients.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var list allowlist
	if err := json.Unmarshal(data, &list); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}

	// public key → name, for a fast lookup
	keys := make(map[string]string)
	for _, c := range list.Clients {
		keys[string(c.Public)] = c.Name
	}

	allowed := func(pub []byte) bool {
		_, ok := keys[string(pub)]
		return ok
	}
	return allowed, nil
}
