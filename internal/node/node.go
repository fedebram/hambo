package node

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"
)

// we can use omitzero for time... but i think it is good like that.

type Node struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	LastSeenAt *time.Time `json:"last_seen_at,omitempty"`
}

const heartbeatMaxAge = 90 * time.Second

func (n Node) IsReachable(now time.Time) bool {
	return n.LastSeenAt != nil &&
		now.Before(n.LastSeenAt.Add(heartbeatMaxAge))
}

func GenerateNodeID() (string, error) {
	idBytes := make([]byte, 16)
	// panic..
	if _, err := rand.Read(idBytes); err != nil {
		return "", fmt.Errorf("generate node ID: %w", err)
	}
	nodeID := hex.EncodeToString(idBytes)
	return nodeID, nil
}
