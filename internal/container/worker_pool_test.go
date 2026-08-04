package container

import (
	"context"
	"errors"
	"testing"
	"testing/synctest"
)

func TestRunWorkerPoolStopsOnContextCancellation(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		queue := NewMemoryQueue()
		worker := newWorker(
			NewMemoryStore(),
			NewMemoryRuntime(),
			queue,
		)

		done := make(chan struct{})

		go func() {
			runWorkerPool(ctx, 1, worker, func(err error) {
				t.Errorf("unexpected worker error: %v", err)
			})
			close(done)
		}()

		synctest.Wait()

		select {
		case <-done:
			t.Fatal("workers returned before context cancellation")
		default:
		}

		cancel()
		synctest.Wait()

		select {
		case <-done:
		default:
			t.Fatal("workers did not return after context cancellation")
		}
	})
}

func TestRunWorkerPoolReportsWorkerErrors(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		wantErr := errors.New("runtime unavailable")

		ctx, cancel := context.WithTimeout(t.Context(), retryDelay/2)
		defer cancel()

		queue := NewMemoryQueue()
		queue.Add("hello")

		worker := newWorker(
			NewMemoryStore(),
			failingRuntime{err: wantErr},
			queue,
		)

		var gotErr error
		reportCalls := 0

		runWorkerPool(ctx, 1, worker, func(err error) {
			gotErr = err
			reportCalls++
		})

		if reportCalls != 1 {
			t.Fatalf("report calls: %d, want 1", reportCalls)
		}
		if !errors.Is(gotErr, wantErr) {
			t.Fatalf("reported error %v, want %v", gotErr, wantErr)
		}
	})
}

func TestRunWorkerPoolRestartsWorkerAfterError(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		wantErr := errors.New("runtime unavailable")

		ctx, cancel := context.WithTimeout(
			t.Context(),
			retryDelay+retryDelay/2,
		)
		defer cancel()

		queue := NewMemoryQueue()
		queue.Add("hello")

		worker := newWorker(
			NewMemoryStore(),
			failingRuntime{err: wantErr},
			queue,
		)

		reportCalls := 0

		runWorkerPool(ctx, 1, worker, func(err error) {
			if !errors.Is(err, wantErr) {
				t.Errorf("reported error %v, want %v", err, wantErr)
			}
			reportCalls++
		})

		if reportCalls != 2 {
			t.Fatalf("report calls: %d, want 2", reportCalls)
		}
	})
}
