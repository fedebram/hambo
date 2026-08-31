package container

import (
	"context"
	"errors"
	"testing"
)

type taskExitSubscriberFunc func(context.Context) (<-chan RuntimeTaskExit, <-chan error)

func (f taskExitSubscriberFunc) SubscribeTaskExit(ctx context.Context) (<-chan RuntimeTaskExit, <-chan error) {
	return f(ctx)
}

func TestRunWorkerListenerEnqueuesTaskExit(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	taskExitCh := make(chan RuntimeTaskExit, 1)
	taskExitCh <- RuntimeTaskExit{ContainerID: "container-id"}

	runtime := taskExitSubscriberFunc(func(context.Context) (<-chan RuntimeTaskExit, <-chan error) {
		return taskExitCh, make(chan error)
	})

	var got string
	RunWorkerListener(ctx, 0, runtime, enqueuerFunc(func(name string) {
		got = name
		cancel()
	}), func(err error) {
		t.Fatalf("unexpected worker listener error: %v", err)
	})

	if got != "container-id" {
		t.Errorf("got enqueued container %q, want %q", got, "container-id")
	}
}

func TestRunWorkerListenerResubscribesAfterError(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	wantErr := errors.New("subscription failed")
	subscribeCalls := 0

	runtime := taskExitSubscriberFunc(func(context.Context) (<-chan RuntimeTaskExit, <-chan error) {
		subscribeCalls++
		if subscribeCalls == 1 {
			errCh := make(chan error, 1)
			errCh <- wantErr
			return make(chan RuntimeTaskExit), errCh
		}

		taskExitCh := make(chan RuntimeTaskExit, 1)
		taskExitCh <- RuntimeTaskExit{ContainerID: "container-id"}
		return taskExitCh, make(chan error)
	})

	reportedErrors := 0
	var got string
	RunWorkerListener(ctx, 0, runtime, enqueuerFunc(func(name string) {
		got = name
		cancel()
	}), func(err error) {
		reportedErrors++
		if !errors.Is(err, wantErr) {
			t.Errorf("got reported error %v, want %v", err, wantErr)
		}
	})

	if subscribeCalls != 2 {
		t.Errorf("got %d subscribe calls, want 2", subscribeCalls)
	}
	if reportedErrors != 1 {
		t.Errorf("got %d reported errors, want 1", reportedErrors)
	}
	if got != "container-id" {
		t.Errorf("got enqueued container %q, want %q", got, "container-id")
	}
}

func TestRunWorkerListenerReturnsOnContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	runtime := taskExitSubscriberFunc(func(context.Context) (<-chan RuntimeTaskExit, <-chan error) {
		t.Fatal("subscribed after context cancellation")
		return nil, nil
	})

	RunWorkerListener(ctx, 0, runtime, enqueuerFunc(func(string) {
		t.Fatal("container enqueued after context cancellation")
	}), func(err error) {
		t.Fatalf("error reported after context cancellation: %v", err)
	})
}
