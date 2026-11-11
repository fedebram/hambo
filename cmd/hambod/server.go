package main

import (
	"encoding/base64"
	"encoding/hex"
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

	clients, err := loadClients(dataDir)
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
		go serveConn(conn, key, clients, tokens, r)
	}
}

// serveConn secures one connection and serves rpc on it until it ends.
// If the client joined with a token, it is registered first.
func serveConn(conn net.Conn, key noise.DHKey, clients *clientStore, tokens *tokenStore, r *rpc.Router) {
	nc, peer, err := noiseconn.Server(conn, key, clients.allowed, tokens.lookup)
	if err != nil {
		fmt.Println("handshake failed:", conn.RemoteAddr(), err)
		conn.Close()
		return
	}

	// a new worker joined with a token: register it before serving
	if peer.TokenID != nil {
		if err := registerWorker(clients, tokens, peer); err != nil {
			fmt.Println("join failed:", conn.RemoteAddr(), err)
			nc.Close()
			return
		}
	}

	sess := muxer.Server(nc)
	defer sess.Close()

	r.Serve(sess)
}

// registerWorker uses up the token a worker joined with,
// then adds the worker's key to the allowlist.
func registerWorker(clients *clientStore, tokens *tokenStore, peer noiseconn.Peer) error {
	ok, err := tokens.consume(peer.TokenID)
	if err != nil {
		return err
	}
	if !ok {
		return errors.New("token already used")
	}

	name := workerName(peer.Public)
	if err := clients.add(name, peer.Public); err != nil {
		return err
	}
	fmt.Println("worker joined:", name)
	return nil
}

// workerName gives a worker a short readable name from its public key,
// like worker-3f9a2c1d.
func workerName(pub []byte) string {
	return "worker-" + hex.EncodeToString(pub[:4])
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
