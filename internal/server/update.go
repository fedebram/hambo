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

// how long a node has to answer its config before we close its session
const sendTimeout = 5 * time.Second

// updateLater puts the node in the queue.
func (s *Server) updateLater(pub []byte) {
	s.queue.Add(hex.EncodeToString(pub))
}

// updateAllNodes puts every connected node in the queue.
func (s *Server) updateAllNodes() {
	for _, pub := range s.reg.connected() {
		s.updateLater(pub)
	}
}

// runUpdates takes nodes from the queue and updates them, retrying with
// backoff when an update fails. It runs until the queue shuts down.
func (s *Server) runUpdates() {
	for {
		key, shutdown := s.queue.Get()
		if shutdown {
			return
		}
		if err := s.updateNode(key); err != nil {
			fmt.Println("update node:", err)
			s.queue.AddAfter(key, s.backoff.Next(key))
		} else {
			s.backoff.Reset(key)
		}
		s.queue.Done(key)
	}
}

// updateNode brings one node up to date with the store: if it was
// removed, it closes its session.
// Otherwise it sends its WireGuard config.
func (s *Server) updateNode(key string) error {
	pub, err := hex.DecodeString(key)
	if err != nil {
		return err
	}

	// the session is not present? it is disconnected, so nothing to do.
	sess := s.reg.get(pub)
	if sess == nil {
		return nil
	}

	// on every update we read all the nodes: we need the others for the peers.
	// right now it is fine, later we can share one read across the updaters.
	nodes, err := s.st.ListNodes(context.Background())
	if err != nil {
		return fmt.Errorf("%s: %w", workerName(pub), err)
	}

	// the node is not in the store? it was removed, so we close its session.
	
	var node *store.Client
	for i := range nodes {
		if bytes.Equal(nodes[i].PublicKey, pub) {
			node = &nodes[i]
		}
	}
	if node == nil {
		sess.Close()
		return nil
	}

	cfg, err := wgConfigFor(*node, nodes, s.cidr)
	if err != nil {
		return fmt.Errorf("%s: %w", workerName(pub), err)
	}

	// a node that doesn't answer in time gets disconnected
	timer := time.AfterFunc(sendTimeout, func() { sess.Close() })
	defer timer.Stop()

	var answer string
	if err := rpc.Call(sess, "wg.config", cfg, &answer); err != nil {
		return fmt.Errorf("%s: %w", workerName(pub), err)
	}
	return nil
}

// wgConfigFor builds the config of node: its subnet, the cluster range,
// and as peers every other node that has an endpoint.
func wgConfigFor(node store.Client, nodes []store.Client, cidr netip.Prefix) (api.WGConfig, error) {
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
