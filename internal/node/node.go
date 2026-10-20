package node

import (
	"crypto/rand"
	"encoding/hex"
	"time"
)

// we can use omitzero for time... but i think it is good like that.

type Node struct {
	ID              string     `json:"id"`
	Name            string     `json:"name"`
	RuntimeReady    bool       `json:"runtime_ready"`
	LastSeenAt      *time.Time `json:"last_seen_at,omitempty"`
	StatusSessionID string     `json:"status_session_id,omitempty"`
	StatusSequence  uint64     `json:"status_sequence"`
}

const heartbeatMaxAge = 90 * time.Second

func (n Node) IsReachable(now time.Time) bool {
	return n.LastSeenAt != nil &&
		now.Before(n.LastSeenAt.Add(heartbeatMaxAge))
}

func GenerateNodeID() string {
	var id [16]byte
	rand.Read(id[:])
	return hex.EncodeToString(id[:])
}
