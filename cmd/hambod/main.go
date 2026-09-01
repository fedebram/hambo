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
	"path/filepath"
	"syscall"
	"time"

	containerdclient "github.com/containerd/containerd/v2/client"
	"github.com/fedebram/hambo/internal/api"
	"github.com/fedebram/hambo/internal/boltstore"
	cninetwork "github.com/fedebram/hambo/internal/cni"
	"github.com/fedebram/hambo/internal/container"
	containerdruntime "github.com/fedebram/hambo/internal/containerd"
)

const (
	defaultAddress             = "127.0.0.1:8080"
	defaultContainerdAddress   = "/run/containerd/containerd.sock"
	defaultContainerdNamespace = "hambo"
	defaultCNIPluginDir        = "/opt/cni/bin"
	defaultCNIPluginConfDir    = "/var/lib/hambo/cni"
	defaultStorePath           = "/var/lib/hambo/hambo.db"
	workerCount                = 1
	workerListenerRetryDelay   = time.Second
	shutdownGracePeriod        = 5 * time.Second
	containerdConnectTimeout   = 5 * time.Second
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
	listener            net.Listener
	containerdNamespace string
	storePath           string
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

func withContainerdNamespace(namespace string) runOption {
	if namespace == "" {
		panic("hambod: containerd namespace cannot be empty")
	}

	return func(cfg *runConfig) {
		cfg.containerdNamespace = namespace
	}
}

func withStorePath(path string) runOption {
	if path == "" {
		panic("hambod: store path cannot be empty")
	}

	return func(cfg *runConfig) {
		cfg.storePath = path
	}
}

func run(ctx context.Context, options ...runOption) (runErr error) {
	cfg := runConfig{
		containerdNamespace: defaultContainerdNamespace,
		storePath:           defaultStorePath,
	}
	for _, option := range options {
		option(&cfg)
	}

	containerdClient, err := containerdclient.New(
		defaultContainerdAddress,
		containerdclient.WithDefaultNamespace(cfg.containerdNamespace),
	)
	if err != nil {
		return fmt.Errorf("create containerd client: %w", err)
	}
	defer func() {
		if err := containerdClient.Close(); err != nil {
			runErr = errors.Join(runErr, fmt.Errorf("close containerd client: %w", err))
		}
	}()

	connectCtx, cancelConnect := context.WithTimeout(ctx, containerdConnectTimeout)
	_, err = containerdClient.Version(connectCtx)
	cancelConnect()
	if err != nil {
		return fmt.Errorf("connect to containerd: %w", err)
	}

	if err := cninetwork.EnsureDefaultConfig(defaultCNIPluginConfDir); err != nil {
		return fmt.Errorf("ensure default CNI configuration: %w", err)
	}

	netAttacher, err := cninetwork.NewAttacher(defaultCNIPluginDir, defaultCNIPluginConfDir)
	if err != nil {
		return fmt.Errorf("initialize container networking: %w", err)
	}

	// match the cni directory permissions. It is fine to allow others to read and traverse the directory.
	if err := os.MkdirAll(filepath.Dir(cfg.storePath), 0o755); err != nil {
		return fmt.Errorf("create store directory: %w", err)
	}

	store, err := boltstore.Open(cfg.storePath)
	if err != nil {
		return fmt.Errorf("open container store: %w", err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			runErr = errors.Join(runErr, fmt.Errorf("close container store: %w", err))
		}
	}()

	listener := cfg.listener
	if listener == nil {
		var err error
		listener, err = net.Listen("tcp", defaultAddress)
		if err != nil {
			return fmt.Errorf("listen: %w", err)
		}
	}

	queue := container.NewMemoryQueue()
	service := container.NewService(store, queue)
	handler := api.NewServer(service)
	runtime := containerdruntime.NewRuntime(containerdClient)

	workerCtx, stopWorkers := context.WithCancel(ctx)
	workersDone := make(chan struct{})
	go func() {
		defer close(workersDone)
		container.RunWorkerPool(workerCtx, shutdownGracePeriod, workerCount, store, runtime, netAttacher, queue, func(err error) {
			slog.Error("container worker failed", "error", err)
		})
	}()
	workerListenerDone := make(chan struct{})
	go func() {
		defer close(workerListenerDone)
		container.RunWorkerListener(workerCtx, workerListenerRetryDelay, runtime, queue, func(err error) {
			slog.Error("container worker listener failed", "error", err)
		})
	}()

	serverErr := runServer(ctx, listener, handler, shutdownGracePeriod)
	stopWorkers()
	<-workersDone
	<-workerListenerDone

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
