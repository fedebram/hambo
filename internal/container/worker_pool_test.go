package container

import (
	"context"
	"errors"
	"testing"
	"testing/synctest"
	"time"
)

type blockingRuntime struct {
	failingRuntime
	started    chan struct{}
	canceledAt chan time.Time
}

func (r blockingRuntime) CreateContainer(ctx context.Context, _, _ string) error {
	close(r.started)
	<-ctx.Done()
	r.canceledAt <- time.Now()
	return ctx.Err()
}

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
			runWorkerPool(ctx, time.Second, 1, worker, func(err error) {
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

		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		container := Container{Name: "hello", State: StateCreating}
		store := NewMemoryStore()
		if err := store.Create(container); err != nil {
			t.Fatalf("unexpected store create error: %v", err)
		}
		queue := NewMemoryQueue()
		queue.Add(container.Name)

		worker := newWorker(
			store,
			failingRuntime{err: wantErr},
			queue,
		)

		var gotErr error
		reportCalls := 0

		runWorkerPool(ctx, time.Second, 1, worker, func(err error) {
			gotErr = err
			reportCalls++
			cancel()
		})

		if reportCalls != 1 {
			t.Fatalf("report calls: %d, want 1", reportCalls)
		}
		if !errors.Is(gotErr, wantErr) {
			t.Fatalf("reported error %v, want %v", gotErr, wantErr)
		}
	})
}

func TestRunWorkerPoolContinuesAfterWorkerError(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		wantErr := errors.New("runtime unavailable")

		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		store := NewMemoryStore()
		queue := NewMemoryQueue()
		for _, name := range []string{"hello", "database"} {
			container := Container{Name: name, State: StateCreating}
			if err := store.Create(container); err != nil {
				t.Fatalf("unexpected store create error: %v", err)
			}
			queue.Add(container.Name)
		}

		worker := newWorker(
			store,
			failingRuntime{err: wantErr},
			queue,
		)

		reportCalls := 0

		runWorkerPool(ctx, time.Second, 1, worker, func(err error) {
			if !errors.Is(err, wantErr) {
				t.Errorf("reported error %v, want %v", err, wantErr)
			}
			reportCalls++
			if reportCalls == 2 {
				cancel()
			}
		})

		if reportCalls != 2 {
			t.Fatalf("report calls: %d, want 2", reportCalls)
		}
	})
}

func TestRunWorkerPoolCancelsActiveWorkAfterGracePeriod(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		const gracePeriod = time.Second

		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		container := Container{Name: "hello", State: StateCreating}
		store := NewMemoryStore()
		if err := store.Create(container); err != nil {
			t.Fatalf("unexpected store create error: %v", err)
		}

		queue := NewMemoryQueue()
		queue.Add(container.Name)

		runtime := blockingRuntime{
			started:    make(chan struct{}),
			canceledAt: make(chan time.Time, 1),
		}
		worker := newWorker(store, runtime, queue)

		var reportedErr error
		reportCalls := 0
		go runWorkerPool(ctx, gracePeriod, 1, worker, func(err error) {
			reportedErr = err
			reportCalls++
		})

		synctest.Wait()
		select {
		case <-runtime.started:
		default:
			t.Fatal("runtime operation did not start")
		}

		shutdownStarted := time.Now()
		cancel()
		canceledAt := <-runtime.canceledAt
		if elapsed := canceledAt.Sub(shutdownStarted); elapsed < gracePeriod {
			t.Fatalf(
				"runtime context cancelled after %v, want at least %v",
				elapsed,
				gracePeriod,
			)
		}

		synctest.Wait()

		if reportCalls != 1 {
			t.Fatalf("report calls: %d, want 1", reportCalls)
		}
		if !errors.Is(reportedErr, context.Canceled) {
			t.Fatalf("reported error %v, want %v", reportedErr, context.Canceled)
		}
	})
}
