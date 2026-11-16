package api

import (
	"net/netip"
	"time"
)

// JoinRequest is what a new worker sends to join.
type JoinRequest struct {
	ID       []byte `json:"id"`
	Secret   []byte `json:"secret"`
	WGPublic []byte `json:"wg_public"`
}

// Node is one machine that joined the server.
type Node struct {
	Name      string    `json:"name"`
	Role      string    `json:"role"`
	Connected bool      `json:"connected"`
	Subnet    string    `json:"subnet"`
	JoinedAt  time.Time `json:"joined_at"`
}

// WGConfig is the WireGuard config the server sends to a worker.
type WGConfig struct {
	Subnet      netip.Prefix `json:"subnet"`       // the node's subnet, like 10.100.1.0/24
	ClusterCIDR netip.Prefix `json:"cluster_cidr"` // the cluster's range, like 10.100.0.0/16
}
