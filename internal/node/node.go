package node

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

type Node struct {
	ID   string `json:"id"`
	Name string `json:"name"`
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
