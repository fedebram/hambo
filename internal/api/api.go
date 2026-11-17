package api

import (
	"net/netip"
	"time"
)

// WGPort is the UDP port wg0 listens on, on every node.
const WGPort = 51820

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
	Peers       []WGPeer     `json:"peers"`        // the other nodes
}

// WGPeer is another node of the cluster.
type WGPeer struct {
	PublicKey []byte         `json:"public_key"` // its wireguard public key
	Endpoint  netip.AddrPort `json:"endpoint"`   // where its wg0 listens, like 192.168.1.20:51820
	Subnet    netip.Prefix   `json:"subnet"`     // its subnet, like 10.100.2.0/24
}

type ContainerSpec struct {
	Name  string `json:"name" yaml:"name"`
	Image string `json:"image" yaml:"image"`
}

type Container struct {
	ID        string        `json:"id"`
	Spec      ContainerSpec `json:"spec"`
	CreatedAt time.Time     `json:"created_at"`
}
