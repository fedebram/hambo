package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/fedebram/hambo/internal/api"
	"github.com/fedebram/hambo/internal/container"
)

const (
	defaultAddress = "127.0.0.1:8080"
	workerCount    = 1
)

func main() {
	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()

	slog.Info(
		"starting hambod",
		"address", "http://"+defaultAddress,
		"workers", workerCount,
	)

	shutdownLogged := make(chan struct{})
	go func() {
		<-ctx.Done()
		slog.Info("shutting down hambod")
		close(shutdownLogged)
	}()

	err := run(ctx)
	// here we sync logging... otherwise shutting down log can be printed after run returned.
	if ctx.Err() != nil {
		<-shutdownLogged
	}

	if err != nil {
		slog.Error("hambod stopped with an error", "error", err)
		os.Exit(1)
	}

	slog.Info("hambod stopped")
	slog.Info("bye bye")
}

type runConfig struct {
	listener net.Listener
}

type runOption func(*runConfig)

func withListener(listener net.Listener) runOption {
	if listener == nil {
		panic("hambod: listener cannot be nil")
	}

	return func(cfg *runConfig) {
		cfg.listener = listener
	}
}

func run(ctx context.Context, options ...runOption) error {
	cfg := runConfig{}
	for _, option := range options {
		option(&cfg)
	}

	listener := cfg.listener
	if listener == nil {
		var err error
		listener, err = net.Listen("tcp", defaultAddress)
		if err != nil {
			return fmt.Errorf("listen: %w", err)
		}
	}

	queue := container.NewMemoryQueue()
	store := container.NewMemoryStore()
	service := container.NewService(store, queue)
	handler := api.NewServer(service)
	runtime := container.NewMemoryRuntime()

	workerCtx, stopWorkers := context.WithCancel(ctx)
	workersDone := make(chan struct{})
	go func() {
		defer close(workersDone)
		container.RunWorkerPool(workerCtx, workerCount, store, runtime, queue, func(err error) {
			slog.Error("container worker failed", "error", err)
		})
	}()

	serverErr := runServer(ctx, listener, handler, 5*time.Second)
	stopWorkers()
	<-workersDone

	return serverErr
}

func runServer(ctx context.Context, listener net.Listener, handler http.Handler, gracePeriod time.Duration) error {
	server := &http.Server{
		Handler: handler,
	}

	serveErrCh := make(chan error, 1)

	go func() {
		serveErrCh <- server.Serve(listener)
	}()

	select {
	case err := <-serveErrCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err

	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), gracePeriod)
		defer cancel()

		shutdownErr := server.Shutdown(shutdownCtx)
		var closeErr error
		if shutdownErr != nil {
			closeErr = server.Close()
		}
		serveErr := <-serveErrCh
		if errors.Is(serveErr, http.ErrServerClosed) {
			serveErr = nil
		}

		return errors.Join(shutdownErr, closeErr, serveErr)
	}
}
