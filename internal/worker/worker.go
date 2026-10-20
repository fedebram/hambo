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
	watchRetryDelay         = 10 * time.Second
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
	// we can use err group. but maybe is better like that...
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	done := make(chan error, 2)

	go func() {
		done <- w.runHeartbeat(runCtx)
	}()

	go func() {
		done <- w.runWatch(runCtx)
	}()

	firstErr := <-done
	cancel()
	secondErr := <-done

	return errors.Join(firstErr, secondErr)
}

func (w *Worker) runHeartbeat(ctx context.Context) error {
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

func (w *Worker) runWatch(ctx context.Context) error {
	for {
		if ctx.Err() != nil {
			return nil
		}

		reader, err := w.client.WatchNode(ctx)
		if err == nil {
			for {
				snapshot, readErr := reader.Next()
				if readErr != nil {
					err = readErr
					break
				}

				if ctx.Err() != nil {
					break
				}

				slog.Info(
					"received node assignments",
					"container_count", len(snapshot.Containers),
				)
			}

			// close the old response before reconnecting.
			if closeErr := reader.Close(); closeErr != nil && ctx.Err() == nil {
				slog.Warn("close node watch", "error", closeErr)
			}
		}

		if ctx.Err() != nil {
			return nil
		}

		if errors.Is(err, errdefs.ErrUnauthenticated) ||
			errors.Is(err, errdefs.ErrPermissionDenied) ||
			errors.Is(err, errdefs.ErrNotFound) {
			return fmt.Errorf("watch node assignments: %w", err)
		}

		slog.Warn("node watch ended, reconnecting", "error", err)

		timer := time.NewTimer(watchRetryDelay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil
		case <-timer.C:
		}
	}
}
