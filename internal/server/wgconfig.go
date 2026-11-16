package server

import (
	"context"
	"fmt"

	"github.com/fedebram/hambo/internal/api"
	"github.com/fedebram/hambo/internal/muxer"
	"github.com/fedebram/hambo/internal/rpc"
)

// sendWGConfig sends the node its subnet and the cluster range.
// If it fails, it closes the session.
func (s *Server) sendWGConfig(ctx context.Context, pub []byte, sess *muxer.Session) {
	subnet, err := s.st.Subnet(ctx, pub)
	if err != nil {
		fmt.Println("wg config:", workerName(pub), err)
		sess.Close()
		return
	}

	cfg := api.WGConfig{Subnet: subnet, ClusterCIDR: s.cidr}
	var answer string
	if err := rpc.Call(sess, "wg.config", cfg, &answer); err != nil {
		fmt.Println("wg config:", workerName(pub), err)
		sess.Close()
	}
}
