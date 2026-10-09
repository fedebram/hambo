package main

import (
	"context"
	"errors"
	"net"
	"net/http"
	"testing"
	"time"
)

func TestRunServerStopsOnCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	t.Cleanup(func() {
		cancel()
		_ = listener.Close()
	})

	errCh := make(chan error, 1)
	go func() {
		errCh <- runServer(ctx, listener, http.NotFoundHandler(), time.Second)
	}()

	cancel()

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("run server returned an error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("run server did not stop after cancellation")
	}
}

func TestRunServerStopsAfterGracePeriod(t *testing.T) {
	const gracePeriod = 100 * time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())
	handlerStarted := make(chan struct{})
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(handlerStarted)
		// this is an handler that remains active, after the grace period server.close is called, so the request context done unblocks.
		<-r.Context().Done()
	})

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	t.Cleanup(func() {
		cancel()
		_ = listener.Close()
	})

	runErr := make(chan error, 1)
	go func() {
		runErr <- runServer(ctx, listener, handler, gracePeriod)
	}()

	getErr := make(chan error, 1)
	go func() {
		resp, err := http.Get("http://" + listener.Addr().String())
		if resp != nil {
			defer resp.Body.Close()
		}
		getErr <- err
	}()

	select {
	case <-handlerStarted:
	case <-time.After(time.Second):
		t.Fatal("handler did not start")
	}

	shutdownStarted := time.Now()
	cancel()

	select {
	case err := <-runErr:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("run server returned %v, want context deadline exceeded", err)
		}
		if elapsed := time.Since(shutdownStarted); elapsed < gracePeriod {
			t.Fatalf("run server stopped after %v, before grace period %v", elapsed, gracePeriod)
		}
	case <-time.After(time.Second):
		t.Fatal("run server did not stop after grace period")
	}

	// we can't check directly that the server is closed after the grace period...

	select {
	case err := <-getErr:
		if err == nil {
			t.Fatal("get returned no error after the connection was closed")
		}
	case <-time.After(time.Second):
		t.Fatal("get did not stop after the connection was closed")
	}
}

func TestRunServerWaitsForActiveRequestWithinGracePeriod(t *testing.T) {
	const gracePeriod = 500 * time.Millisecond

	ctx, cancel := context.WithCancel(context.Background())
	handlerStarted := make(chan struct{})
	releaseHandler := make(chan struct{})
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(handlerStarted)
		<-releaseHandler
		w.WriteHeader(http.StatusNoContent)
	})

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	t.Cleanup(func() {
		cancel()
		_ = listener.Close()
	})

	runErr := make(chan error, 1)
	go func() {
		runErr <- runServer(ctx, listener, handler, gracePeriod)
	}()

	getErr := make(chan error, 1)
	go func() {
		resp, err := http.Get("http://" + listener.Addr().String())
		if resp != nil {
			defer resp.Body.Close()
		}
		getErr <- err
	}()

	select {
	case <-handlerStarted:
	case <-time.After(time.Second):
		t.Fatal("handler did not start")
	}

	// core logic of runServer: if all the handlers return before the grace period we exit clean.

	shutdownStarted := time.Now()
	cancel()
	time.AfterFunc(25*time.Millisecond, func() {
		close(releaseHandler)
	})

	select {
	case err := <-runErr:
		if err != nil {
			t.Fatalf("run server returned an error: %v", err)
		}
		if elapsed := time.Since(shutdownStarted); elapsed >= gracePeriod {
			t.Fatalf("run server stopped after %v, want less than grace period %v", elapsed, gracePeriod)
		}
	case <-time.After(time.Second):
		t.Fatal("run server did not stop after the active request completed")
	}

	select {
	case err := <-getErr:
		if err != nil {
			t.Fatalf("get returned an error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("get did not complete")
	}
}
