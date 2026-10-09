package worker

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/fedebram/hambo/client"
)

type Worker struct {
	client *client.Client
}

func NewWorker(apiClient *client.Client) *Worker {
	if apiClient == nil {
		panic("worker: API client cannot be nil")
	}
	return &Worker{client: apiClient}
}

func (w *Worker) Run(ctx context.Context) error {
	health, err := w.client.Health(ctx)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return nil
		}
		return fmt.Errorf("check server health: %w", err)
	}
	if health.Status != "ok" {
		return fmt.Errorf("server is not healthy: status %q", health.Status)
	}

	slog.Info("hambo server is healthy")

	// Registration and work handling will be added here next.
	<-ctx.Done()
	return nil
}
