package worker

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/fedebram/hambo/client"
	"github.com/fedebram/hambo/errdefs"
)

// relaxed intervals
const (
	heartbeatInterval       = 30 * time.Second
	heartbeatRequestTimeout = 5 * time.Second
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
	ticker := time.NewTicker(heartbeatInterval)
	defer ticker.Stop()

	for {
		requestCtx, cancel := context.WithTimeout(
			ctx, heartbeatRequestTimeout,
		)
		err := w.client.ReportNodeStatus(requestCtx)
		cancel()

		if ctx.Err() != nil {
			return nil
		}

		if err != nil {
			if errors.Is(err, errdefs.ErrUnauthenticated) ||
				errors.Is(err, errdefs.ErrPermissionDenied) ||
				errors.Is(err, errdefs.ErrNotFound) {
				return fmt.Errorf("report node status: %w", err)
			}

			slog.Warn("could not report node status", "error", err)
		}

		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}
