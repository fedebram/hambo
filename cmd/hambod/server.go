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

	"github.com/fedebram/hambo/internal/api"
	"github.com/fedebram/hambo/internal/rpc"
	"github.com/fedebram/hambo/internal/store"
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

	st, err := store.Open(filepath.Join(dataDir, "hambo.db"))
	if err != nil {
		return err
	}
	defer st.Close()

	adminOnly := requireRole(store.RoleAdmin)
	anyone := requireRole(store.RoleAdmin, store.RoleWorker)

	// the methods clients can call
	r := rpc.NewRouter()
	r.Handle("ping", anyone(func(ctx context.Context, s *rpc.Stream) error {
		return s.Send("pong")
	}))
	r.Handle("tokens.create", adminOnly(func(ctx context.Context, s *rpc.Stream) error {
		id, secret, err := st.CreateToken(ctx, tokenTTL)
		if err != nil {
			return err
		}
		return s.Send(joinToken(key.Public, id, secret))
	}))
	r.Handle("join", requireRole(store.RoleUnknown, store.RoleWorker)(func(ctx context.Context, s *rpc.Stream) error {
		p := peerFrom(ctx)
		if p.Role == store.RoleWorker {
			return s.Send("already joined")
		}

		var req api.JoinRequest
		if err := s.Recv(&req); err != nil {
			return err
		}
		name := workerName(p.PubKey)
		ok, err := st.Join(ctx, req.ID, req.Secret, store.Client{PublicKey: p.PubKey, Name: name, Role: store.RoleWorker})
		if err != nil {
			return err
		}
		if !ok {
			return rpc.Errorf("invalid or used token")
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

	reg := newRegistry()
	for {
		conn, err := ln.Accept()
		if err != nil {
			return err
		}
		go serveConn(conn, key, st, reg, r)
	}
}

const (
	// how long an unknown key may stay connected: enough to call join
	unknownTimeout = 30 * time.Second
	// how long a join token stays valid
	tokenTTL = 24 * time.Hour
)

// serveConn runs the handshake and serves rpc until the session ends.
func serveConn(conn net.Conn, key noise.DHKey, st *store.Store, reg *registry, r *rpc.Router) {
	sess, pub, err := transport.Accept(conn, key)
	if err != nil {
		fmt.Println("handshake failed:", conn.RemoteAddr(), err)
		return
	}
	defer sess.Close()

	ctx := context.Background()
	role, err := st.Role(ctx, pub)
	if err != nil {
		fmt.Println("role lookup failed:", conn.RemoteAddr(), err)
		return
	}
	p := peer{PubKey: pub, Role: role}
	if p.Role == store.RoleUnknown {
		time.AfterFunc(unknownTimeout, func() { sess.Close() })
	}
	if p.Role == store.RoleWorker {
		reg.add(pub, sess)
		defer reg.remove(pub, sess)
	}

	r.Serve(withPeer(ctx, p), sess)
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
