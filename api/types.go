package api

type HealthResponse struct {
	Status string `json:"status"`
}

type ErrorResponse struct {
	Message string            `json:"message"`
	Fields  map[string]string `json:"fields,omitempty"`
}
