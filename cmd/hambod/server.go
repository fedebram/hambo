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
	"github.com/fedebram/hambo/internal/muxer"
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
	sf, err := loadServerFile(dataDir)
	if err != nil {
		return err
	}
	key := noise.DHKey{Private: sf.Private, Public: sf.Public}
	fmt.Println("server public key:", base64.StdEncoding.EncodeToString(key.Public))
	fmt.Println("cluster cidr:", sf.CIDR)

	st, err := store.Open(filepath.Join(dataDir, "hambo.db"))
	if err != nil {
		return err
	}
	defer st.Close()

	reg := newRegistry()

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
	r.Handle("nodes.list", adminOnly(func(ctx context.Context, s *rpc.Stream) error {
		clients, err := st.ListNodes(ctx)
		if err != nil {
			return err
		}
		nodes := []api.Node{}
		for _, c := range clients {
			nodes = append(nodes, api.Node{
				Name:      c.Name,
				Role:      string(c.Role),
				Connected: reg.get(c.PublicKey) != nil,
				JoinedAt:  c.CreatedAt,
			})
		}
		return s.Send(nodes)
	}))
	r.Handle("nodes.remove", adminOnly(func(ctx context.Context, s *rpc.Stream) error {
		var name string
		if err := s.Recv(&name); err != nil {
			return err
		}
		pub, found, err := st.RemoveNode(ctx, name)
		if err != nil {
			return err
		}
		if !found {
			return rpc.Errorf("no node named %s", name)
		}
		if sess := reg.get(pub); sess != nil {
			sess.Close()
		}
		fmt.Println("node removed:", name)
		return s.Send("removed")
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
		if !addWorker(ctx, st, reg, pub, sess) {
			return // removed while connecting
		}
		defer reg.remove(pub, sess)
	}

	r.Serve(withPeer(ctx, p), sess)
}

// addWorker puts the worker in the registry. It reports false if the
// worker was removed while it was connecting.
func addWorker(ctx context.Context, st *store.Store, reg *registry, pub []byte, sess *muxer.Session) bool {
	reg.add(pub, sess)
	// nodes.remove can delete the node between the role check in serveConn
	// and adding the worker to the registry.
	// we check the role again to see if it was deleted.
	role, err := st.Role(ctx, pub)
	if err != nil || role != store.RoleWorker {
		reg.remove(pub, sess)
		return false
	}
	return true
}

// workerName gives a worker a short readable name from its public key,
// like worker-3f9a2c1d.
func workerName(pub []byte) string {
	return "worker-" + hex.EncodeToString(pub[:4])
}

// loadServerFile reads the server's key and cluster range from server.json.
func loadServerFile(dataDir string) (serverFile, error) {
	path := filepath.Join(dataDir, "server.json")
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return serverFile{}, fmt.Errorf("not initialized: %s not found, run: hambod init", path)
	}
	if err != nil {
		return serverFile{}, err
	}

	var sf serverFile
	if err := json.Unmarshal(data, &sf); err != nil {
		return serverFile{}, fmt.Errorf("%s: %w", path, err)
	}
	return sf, nil
}
