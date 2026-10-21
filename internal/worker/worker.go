package worker

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/fedebram/hambo/api"
	"github.com/fedebram/hambo/client"
	"github.com/fedebram/hambo/errdefs"
	dockerclient "github.com/moby/moby/client"
)

// relaxed intervals
const (
	heartbeatInterval       = 30 * time.Second
	heartbeatRequestTimeout = 5 * time.Second
	watchRetryDelay         = 10 * time.Second
)

type Worker struct {
	apiClient *client.Client
	docker    *dockerclient.Client
	sessionID string
	sequence  uint64
}

func NewWorker(apiClient *client.Client, docker *dockerclient.Client) *Worker {
	if apiClient == nil {
		panic("worker: API client cannot be nil")
	}
	if docker == nil {
		panic("worker: docker client cannot be nil")
	}
	return &Worker{
		apiClient: apiClient,
		docker:    docker,
	}
}

func (w *Worker) Run(ctx context.Context) error {
	requestCtx, cancelSession := context.WithTimeout(ctx, heartbeatRequestTimeout)
	session, err := w.apiClient.CreateNodeSession(requestCtx)
	cancelSession()
	if ctx.Err() != nil {
		return nil
	}
	if err != nil {
		return fmt.Errorf("create node session: %w", err)
	}
	w.sessionID = session.SessionID
	w.sequence = 1

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

	report := api.ReportNodeStatusRequest{
		SessionID:  w.sessionID,
		Containers: []api.ContainerStatusReport{},
	}

	for {
		// this is a defensive check
		if w.sequence == 0 {
			return errors.New("node status sequence exhausted")
		}

		checkCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		_, err := w.docker.Ping(checkCtx, dockerclient.PingOptions{})
		cancel()

		if ctx.Err() != nil {
			return nil
		}

		report.Sequence = w.sequence
		report.RuntimeReady = err == nil

		if err != nil {
			slog.Warn("container runtime unavailable", "error", err)
		}
		requestCtx, cancel := context.WithTimeout(ctx, heartbeatRequestTimeout)
		err = w.apiClient.ReportNodeStatus(requestCtx, report)
		cancel()

		if ctx.Err() != nil {
			return nil
		}
		if err != nil {
			if errors.Is(err, errdefs.ErrConflict) ||
				errors.Is(err, errdefs.ErrInvalidArgument) ||
				errors.Is(err, errdefs.ErrUnauthenticated) ||
				errors.Is(err, errdefs.ErrPermissionDenied) ||
				errors.Is(err, errdefs.ErrNotFound) {
				return fmt.Errorf("report node status: %w", err)
			}
			// transient failure
			slog.Warn("could not report node status", "error", err)
		}
		w.sequence++

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

		reader, err := w.apiClient.WatchNode(ctx)
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
