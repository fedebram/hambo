package image

import (
	"sync"

	client "github.com/containerd/containerd/v2/client"
)

type Service struct {
	client *client.Client
	mu     sync.Mutex
}

func NewService(client *client.Client) *Service {
	return &Service{client: client}
}
