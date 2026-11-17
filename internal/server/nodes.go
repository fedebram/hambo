package server

import (
	"context"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/fedebram/hambo/internal/api"
	"github.com/fedebram/hambo/internal/rpc"
	"github.com/fedebram/hambo/internal/store"
)

// joinNode uses up the token and adds the node with a free subnet.
func (s *Server) joinNode(ctx context.Context, pub []byte, req api.JoinRequest) error {
	// we make joins serialized instead of using store transactions.

	s.joinMu.Lock()
	defer s.joinMu.Unlock()

	if len(req.WGPublic) != 32 {
		return rpc.Errorf("invalid wireguard key")
	}

	token, found, err := s.st.GetToken(ctx, req.ID)
	if err != nil {
		return err
	}
	if !found || subtle.ConstantTimeCompare(token.Secret, req.Secret) != 1 || !time.Now().Before(token.ExpiresAt) {
		return rpc.Errorf("invalid or used token")
	}

	taken, err := s.st.ListSubnets(ctx)
	if err != nil {
		return err
	}
	subnet, err := freeSubnet(s.cidr, taken)
	if errors.Is(err, errNoFreeSubnet) {
		return rpc.Errorf("the cluster is full: no free subnet in %s", s.cidr)
	}
	if err != nil {
		return err
	}

	node := store.Node{
		PublicKey: pub,
		Name:      workerName(pub),
		CreatedAt: time.Now(),
		WGPublic:  req.WGPublic,
		Subnet:    subnet.String(),
	}
	used, err := s.st.UseToken(ctx, req.ID, node)
	if errors.Is(err, store.ErrConflict) {
		return rpc.Errorf("a node with this key or name already exists")
	}
	if err != nil {
		return err
	}
	if !used {
		return rpc.Errorf("invalid or used token")
	}
	fmt.Println("node joined:", node.Name, node.Subnet)
	return nil
}

// listNodes returns the nodes, and whether each one is connected.
func (s *Server) listNodes(ctx context.Context) ([]api.Node, error) {
	stored, err := s.st.ListNodes(ctx)
	if err != nil {
		return nil, err
	}
	nodes := []api.Node{}
	for _, n := range stored {
		nodes = append(nodes, api.Node{
			Name:      n.Name,
			Connected: s.sessions.isConnected(n.PublicKey),
			Subnet:    n.Subnet,
			JoinedAt:  n.CreatedAt,
		})
	}
	return nodes, nil
}

// removeNode deletes the node and its containers.
func (s *Server) removeNode(ctx context.Context, name string) error {
	found, err := s.st.RemoveNode(ctx, name)
	if err != nil {
		return err
	}
	if !found {
		return rpc.Errorf("no node named %s", name)
	}
	fmt.Println("node removed:", name)
	s.onNodeRemoved()
	return nil
}

// workerName gives a node a short readable name from its public key,
// like worker-3f9a2c1d.
func workerName(pub []byte) string {
	return "worker-" + hex.EncodeToString(pub[:4])
}
