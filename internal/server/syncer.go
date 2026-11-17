package server

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"net/netip"
	"time"

	"github.com/fedebram/hambo/internal/api"
	"github.com/fedebram/hambo/internal/rpc"
	"github.com/fedebram/hambo/internal/store"
)

// how long a node has to answer before we close its session
const sendTimeout = 5 * time.Second

// syncNode makes the node match the store: a removed node gets
// disconnected, the others get their wg config and their containers.
func (s *Server) syncNode(key string) error {
	pub, err := hex.DecodeString(key)
	if err != nil {
		return err
	}
	ctx := context.Background()

	// not connected: it gets everything when it connects
	sess := s.sessions.get(pub)
	if sess == nil {
		return nil
	}

	// we read every node: the others are its peers.
	nodes, err := s.st.ListNodes(ctx)
	if err != nil {
		return err
	}
	var node *store.Node
	for i := range nodes {
		if bytes.Equal(nodes[i].PublicKey, pub) {
			node = &nodes[i]
		}
	}
	if node == nil {
		s.disconnect(sess, workerName(pub), "not in the store anymore")
		return nil
	}

	cfg, err := wgConfigFor(*node, nodes, s.cidr)
	if err != nil {
		return fmt.Errorf("%s: %w", node.Name, err)
	}
	stored, err := s.st.ContainersAssignedTo(ctx, pub)
	if err != nil {
		return fmt.Errorf("%s: %w", node.Name, err)
	}
	var containers []api.NodeContainer
	for _, c := range stored {
		containers = append(containers, api.NodeContainer{ID: c.ID, Spec: c.Spec})
	}

	timer := time.AfterFunc(sendTimeout, func() {
		s.disconnect(sess, node.Name, "didn't answer in time")
	})
	defer timer.Stop()

	var answer string
	if err := rpc.Call(sess, "wg.config", cfg, &answer); err != nil {
		return fmt.Errorf("%s: %w", node.Name, err)
	}
	if err := rpc.Call(sess, "containers.sync", containers, &answer); err != nil {
		return fmt.Errorf("%s: %w", node.Name, err)
	}
	return nil
}

// wgConfigFor builds the config of node: its subnet, the cluster range,
// and as peers every other node that has an endpoint.
func wgConfigFor(node store.Node, nodes []store.Node, cidr netip.Prefix) (api.WGConfig, error) {
	subnet, err := netip.ParsePrefix(node.Subnet)
	if err != nil {
		return api.WGConfig{}, err
	}
	cfg := api.WGConfig{Subnet: subnet, ClusterCIDR: cidr}

	for _, n := range nodes {
		if bytes.Equal(n.PublicKey, node.PublicKey) {
			continue // itself
		}
		if n.Endpoint == "" {
			continue // it never connected: we don't know where to reach it
		}
		endpoint, err := netip.ParseAddrPort(n.Endpoint)
		if err != nil {
			return api.WGConfig{}, err
		}
		peerSubnet, err := netip.ParsePrefix(n.Subnet)
		if err != nil {
			return api.WGConfig{}, err
		}
		cfg.Peers = append(cfg.Peers, api.WGPeer{
			PublicKey: n.WGPublic,
			Endpoint:  endpoint,
			Subnet:    peerSubnet,
		})
	}
	return cfg, nil
}
