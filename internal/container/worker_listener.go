package container

import (
	"context"
	"errors"
	"fmt"
	"time"
)

type RuntimeEventSource interface {
	SubscribeTaskExit(ctx context.Context) (<-chan RuntimeTaskExit, <-chan error)
}

type RuntimeTaskExit struct {
	ContainerID string
	ExitCode    uint32
	ExitedAt    time.Time
}

// If we standardize workers and worker pool?

func RunWorkerListener(
	ctx context.Context,
	retryDelay time.Duration,
	runtime RuntimeEventSource,
	queue enqueuer,
	reportError func(error),
) {
	for ctx.Err() == nil {
		err := listenTaskExits(ctx, runtime, queue)
		if err == nil {
			return
		}

		reportError(err)

		// TODO: retry with backoff
		timer := time.NewTimer(retryDelay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}

func listenTaskExits(ctx context.Context, runtime RuntimeEventSource, queue enqueuer) error {
	taskExitCh, errCh := runtime.SubscribeTaskExit(ctx)

	for {
		select {
		case <-ctx.Done():
			return nil
		case taskExit, ok := <-taskExitCh:
			if !ok {
				// we close the task exit channel and the error channel.
				// So we continue to select the error channel.
				taskExitCh = nil
				continue
			}
			queue.Add(taskExit.ContainerID)
		case err, ok := <-errCh:
			if ctx.Err() != nil {
				return nil
			}
			if !ok || err == nil {
				return errors.New("runtime task exit subscription ended unexpectedly")
			}
			return fmt.Errorf("listen for runtime task exit events: %w", err)
		}
	}
}
