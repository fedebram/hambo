package main

import (
	"context"
	"fmt"
	"time"

	"github.com/moby/moby/client"
)

func newDockerClient(ctx context.Context) (*client.Client, error) {
	docker, err := client.New(client.FromEnv)
	if err != nil {
		return nil, fmt.Errorf("create Docker client: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	_, err = docker.Ping(pingCtx, client.PingOptions{
		NegotiateAPIVersion: true,
	})
	cancel()
	if err != nil {
		docker.Close()
		return nil, fmt.Errorf("reach Docker: %w", err)
	}

	return docker, nil
}
