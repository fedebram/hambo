package main

import (
	"context"
	"slices"

	"github.com/fedebram/hambo/internal/rpc"
)

// peer is the client of a session: its public key and its role.
type peer struct {
	PubKey []byte // verified by the handshake
	Role   role
}

type peerKey struct{}

// withPeer returns a ctx that carries p.
func withPeer(ctx context.Context, p peer) context.Context {
	return context.WithValue(ctx, peerKey{}, p)
}

// peerFrom returns the peer in ctx, or an unknown one.
func peerFrom(ctx context.Context) peer {
	p, _ := ctx.Value(peerKey{}).(peer)
	return p
}

// requireRole lets through only callers with one of these roles.
// The others get "unknown method".
func requireRole(roles ...role) func(rpc.Handler) rpc.Handler {
	return func(next rpc.Handler) rpc.Handler {
		return func(ctx context.Context, s *rpc.Stream) error {
			if !slices.Contains(roles, peerFrom(ctx).Role) {
				return rpc.Errorf("unknown method")
			}
			return next(ctx, s)
		}
	}
}
