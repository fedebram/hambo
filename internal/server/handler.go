package server

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/fedebram/hambo/internal/api"
	"github.com/fedebram/hambo/internal/jointoken"
	"github.com/fedebram/hambo/internal/rpc"
	"github.com/fedebram/hambo/internal/store"
)

// how long a join token stays valid
const tokenTTL = 24 * time.Hour

// routes returns the router with the methods clients can call.
func (s *Server) routes() *rpc.Router {
	admin := s.requireRole(store.RoleAdmin)
	anyone := s.requireRole(store.RoleAdmin, store.RoleWorker)
	newcomer := s.requireRole(store.RoleUnknown, store.RoleWorker)

	r := rpc.NewRouter()
	r.Handle("ping", anyone(s.ping))
	r.Handle("tokens.create", admin(s.createToken))
	r.Handle("join", newcomer(s.join))
	r.Handle("nodes.list", admin(s.listNodes))
	r.Handle("nodes.remove", admin(s.removeNode))
	return r
}

func (s *Server) ping(ctx context.Context, stream *rpc.Stream) error {
	return stream.Send("pong")
}

func (s *Server) createToken(ctx context.Context, stream *rpc.Stream) error {
	id, secret, err := s.st.CreateToken(ctx, tokenTTL)
	if err != nil {
		return err
	}
	return stream.Send(jointoken.Encode(s.key.Public, id, secret))
}

func (s *Server) join(ctx context.Context, stream *rpc.Stream) error {
	c := callerFrom(ctx)
	if c.Role == store.RoleWorker {
		return stream.Send("already joined")
	}

	var req api.JoinRequest
	if err := stream.Recv(&req); err != nil {
		return err
	}
	if len(req.WGPublic) != 32 {
		return rpc.Errorf("invalid wireguard key")
	}

	name := workerName(c.PubKey)
	node := store.Client{PublicKey: c.PubKey, Name: name, Role: store.RoleWorker, WGPublic: req.WGPublic}
	ok, err := s.st.Join(ctx, req.ID, req.Secret, node, s.cidr)
	if errors.Is(err, store.ErrNoFreeSubnet) {
		return rpc.Errorf("the cluster is full: no free subnet in %s", s.cidr)
	}
	if err != nil {
		return err
	}
	if !ok {
		return rpc.Errorf("invalid or used token")
	}
	fmt.Println("worker joined:", name)
	return stream.Send("joined")
}

func (s *Server) listNodes(ctx context.Context, stream *rpc.Stream) error {
	clients, err := s.st.ListNodes(ctx)
	if err != nil {
		return err
	}
	nodes := []api.Node{}
	for _, c := range clients {
		nodes = append(nodes, api.Node{
			Name:      c.Name,
			Role:      string(c.Role),
			Connected: s.reg.has(c.PublicKey),
			Subnet:    c.Subnet,
			JoinedAt:  c.CreatedAt,
		})
	}
	return stream.Send(nodes)
}

func (s *Server) removeNode(ctx context.Context, stream *rpc.Stream) error {
	var name string
	if err := stream.Recv(&name); err != nil {
		return err
	}
	_, found, err := s.st.RemoveNode(ctx, name)
	if err != nil {
		return err
	}
	if !found {
		return rpc.Errorf("no node named %s", name)
	}
	fmt.Println("node removed:", name)
	// we update all nodes: the update closes its session,
	// and the others drop it from their peers.
	s.updateAllNodes()
	return stream.Send("removed")
}

// workerName gives a worker a short readable name from its public key,
// like worker-3f9a2c1d.
func workerName(pub []byte) string {
	return "worker-" + hex.EncodeToString(pub[:4])
}
