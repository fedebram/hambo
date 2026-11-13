package main

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/fedebram/hambo/internal/rpc"
	"github.com/fedebram/hambo/internal/transport"
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

	adminOnly := requireRole(roleAdmin)
	anyone := requireRole(roleAdmin, roleWorker)

	// the methods clients can call
	r := rpc.NewRouter()
	r.Handle("ping", anyone(func(ctx context.Context, s *rpc.Stream) error {
		return s.Send("pong")
	}))
	r.Handle("tokens.create", adminOnly(func(ctx context.Context, s *rpc.Stream) error {
		id, secret, err := tokens.create()
		if err != nil {
			return err
		}
		return s.Send(joinToken(key.Public, id, secret))
	}))
	r.Handle("join", requireRole(roleUnknown, roleWorker)(func(ctx context.Context, s *rpc.Stream) error {
		p := peerFrom(ctx)
		if p.Role == roleWorker {
			return s.Send("already joined")
		}

		var req joinRequest
		if err := s.Recv(&req); err != nil {
			return err
		}
		ok, err := tokens.consume(req.ID, req.Secret)
		if err != nil {
			return err
		}
		if !ok {
			return rpc.Errorf("invalid or used token")
		}

		name := workerName(p.PubKey)
		if err := clients.add(client{Name: name, Public: p.PubKey, Role: roleWorker}); err != nil {
			return err
		}
		fmt.Println("worker joined:", name)
		return s.Send("joined")
	}))

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
		go serveConn(conn, key, clients, r)
	}
}

// how long an unknown key may stay connected: enough to call join
const unknownTimeout = 30 * time.Second

// serveConn runs the handshake and serves rpc until the session ends.
func serveConn(conn net.Conn, key noise.DHKey, clients *clientStore, r *rpc.Router) {
	sess, pub, err := transport.Accept(conn, key)
	if err != nil {
		fmt.Println("handshake failed:", conn.RemoteAddr(), err)
		return
	}
	defer sess.Close()

	p := peer{PubKey: pub, Role: clients.role(pub)}
	if p.Role == roleUnknown {
		time.AfterFunc(unknownTimeout, func() { sess.Close() })
	}

	r.Serve(withPeer(context.Background(), p), sess)
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
