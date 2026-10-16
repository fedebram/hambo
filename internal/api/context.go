package api

import "context"

type nodeIDContextKey struct{}

func nodeIDFromContext(ctx context.Context) (string, bool) {
	nodeID, ok := ctx.Value(nodeIDContextKey{}).(string)
	return nodeID, ok
}
