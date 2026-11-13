package api

import "time"

// JoinRequest is what a new worker sends to join.
type JoinRequest struct {
	ID     []byte `json:"id"`
	Secret []byte `json:"secret"`
}

// Node is one machine that joined the server.
type Node struct {
	Name      string    `json:"name"`
	Role      string    `json:"role"`
	Connected bool      `json:"connected"`
	JoinedAt  time.Time `json:"joined_at"`
}
