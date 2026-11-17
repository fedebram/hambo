package worker

import (
	"context"
	"fmt"
	"net/netip"

	"github.com/fedebram/hambo/internal/api"
	"github.com/fedebram/hambo/internal/rpc"
	"github.com/fedebram/hambo/internal/wireguard"
)

// routes returns the router with the methods the server can call.
func (w *worker) routes() *rpc.Router {
	r := rpc.NewRouter()
	r.Handle("health", w.health)
	r.Handle("wg.config", w.wgConfig)
	r.Handle("containers.sync", w.containersSync)
	return r
}

// containersSync receives the full list of containers that should run on
// this node. For now it only prints it.
func (w *worker) containersSync(ctx context.Context, stream *rpc.Stream) error {
	var containers []api.NodeContainer
	if err := stream.Recv(&containers); err != nil {
		return err
	}

	fmt.Println("containers:", len(containers))
	for _, c := range containers {
		fmt.Println(" ", c.Spec.Name, c.ID, c.Spec.Image)
	}
	return stream.Send("ok")
}

func (w *worker) health(ctx context.Context, stream *rpc.Stream) error {
	return stream.Send("ok")
}

// wgConfig makes wg0 match the config the server sent.
func (w *worker) wgConfig(ctx context.Context, stream *rpc.Stream) error {
	var cfg api.WGConfig
	if err := stream.Recv(&cfg); err != nil {
		return err
	}

	w.wgMu.Lock()
	defer w.wgMu.Unlock()

	if err := wireguard.SetInterface(w.id.WGPrivate, api.WGPort); err != nil {
		return fmt.Errorf("set wg0: %w", err)
	}
	if err := wireguard.SetAddress(cfg.Subnet, cfg.ClusterCIDR); err != nil {
		return fmt.Errorf("set wg0 address: %w", err)
	}

	var peers []wireguard.Peer
	for _, p := range cfg.Peers {
		peers = append(peers, wireguard.Peer{
			PublicKey:  p.PublicKey,
			Endpoint:   p.Endpoint,
			AllowedIPs: []netip.Prefix{p.Subnet},
		})
	}
	if err := wireguard.SetPeers(peers); err != nil {
		return fmt.Errorf("set wg0 peers: %w", err)
	}

	fmt.Println("subnet:", cfg.Subnet, "peers:", len(peers))
	return stream.Send("ok")
}
