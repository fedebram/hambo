package worker

import (
	"context"
	"fmt"

	"github.com/fedebram/hambo/internal/api"
	"github.com/fedebram/hambo/internal/rpc"
	"github.com/fedebram/hambo/internal/wireguard"
)

// routes returns the router with the methods the server can call.
func (w *worker) routes() *rpc.Router {
	r := rpc.NewRouter()
	r.Handle("health", w.health)
	r.Handle("wg.config", w.wgConfig)
	return r
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
	if err := wireguard.SetInterface(w.id.WGPrivate); err != nil {
		return fmt.Errorf("set wg0: %w", err)
	}
	if err := wireguard.SetAddress(cfg.Subnet, cfg.ClusterCIDR); err != nil {
		return fmt.Errorf("set wg0 address: %w", err)
	}
	fmt.Println("subnet:", cfg.Subnet)
	return stream.Send("ok")
}
