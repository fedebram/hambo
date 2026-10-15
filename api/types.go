package api

import "time"

type HealthResponse struct {
	Status string `json:"status"`
}

type ErrorResponse struct {
	Message string            `json:"message"`
	Fields  map[string]string `json:"fields,omitempty"`
}

type CreateTokenRequest struct {
	TTL string `json:"ttl,omitempty"`
}

type CreateTokenResponse struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
}

type JoinNodeRequest struct {
	Name string `json:"name"`
	CSR  []byte `json:"csr"`
}

type JoinNodeResponse struct {
	NodeID      string `json:"node_id"`
	Certificate string `json:"certificate"`
}
