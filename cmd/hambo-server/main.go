package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
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

	"github.com/fedebram/hambo/internal/api"
	"github.com/fedebram/hambo/internal/store"
)

const (
	defaultAddress      = "127.0.0.1:8080"
	defaultStorePath    = "./data/hambo.db"
	defaultCAFile       = "./certs/ca.crt"
	defaultCertFile     = "certs/server.crt"
	defaultKeyFile      = "certs/server.key"
	shutdownGracePeriod = 5 * time.Second
)

func main() {
	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()

	slog.Info(
		"starting hambo server",
		"address", "https://"+defaultAddress,
	)

	shutdownLogged := make(chan struct{})
	go func() {
		<-ctx.Done()
		slog.Info("shutting down hambo server")
		close(shutdownLogged)
	}()

	err := run(ctx)
	// here we sync logging... otherwise shutting down log can be printed after run returned.
	if ctx.Err() != nil {
		<-shutdownLogged
	}

	if err != nil {
		slog.Error("hambo server stopped with an error", "error", err)
		os.Exit(1)
	}

	slog.Info("hambo server stopped")
	slog.Info("bye bye")
}

type runConfig struct {
	listener  net.Listener
	storePath string
	caFile    string
	certFile  string
	keyFile   string
}

type runOption func(*runConfig)

func withListener(listener net.Listener) runOption {
	if listener == nil {
		panic("hambo server: listener cannot be nil")
	}

	return func(cfg *runConfig) {
		cfg.listener = listener
	}
}

func withStorePath(path string) runOption {
	if path == "" {
		panic("hambo server: store path cannot be empty")
	}

	return func(cfg *runConfig) {
		cfg.storePath = path
	}
}

func withTLSFiles(certFile, keyFile string) runOption {
	if certFile == "" || keyFile == "" {
		panic("hambo server: TLS certificate and key paths cannot be empty")
	}

	return func(cfg *runConfig) {
		cfg.certFile = certFile
		cfg.keyFile = keyFile
	}
}

func withCAFile(path string) runOption {
	if path == "" {
		panic("hambo server: CA certificate path cannot be empty")
	}

	return func(cfg *runConfig) {
		cfg.caFile = path
	}
}

func run(ctx context.Context, options ...runOption) (runErr error) {
	cfg := runConfig{
		storePath: defaultStorePath,
		caFile:    defaultCAFile,
		certFile:  defaultCertFile,
		keyFile:   defaultKeyFile,
	}
	for _, option := range options {
		option(&cfg)
	}

	caPEM, err := os.ReadFile(cfg.caFile)
	if err != nil {
		return fmt.Errorf("read client CA certificate: %w", err)
	}
	clientCAs := x509.NewCertPool()
	if !clientCAs.AppendCertsFromPEM(caPEM) {
		return errors.New("client CA file contains no valid PEM certificates")
	}

	if err := os.MkdirAll(filepath.Dir(cfg.storePath), 0o700); err != nil {
		return fmt.Errorf("create store directory: %w", err)
	}

	store, err := store.Open(cfg.storePath)
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

	handler := api.NewServer()

	serverErr := runServer(ctx, listener, handler, shutdownGracePeriod, cfg.certFile, cfg.keyFile, clientCAs)

	return serverErr
}

func runServer(ctx context.Context, listener net.Listener, handler http.Handler, gracePeriod time.Duration, certFile, keyFile string, clientCAs *x509.CertPool) error {
	server := &http.Server{
		Handler: handler,
		TLSConfig: &tls.Config{
			ClientCAs:  clientCAs,
			ClientAuth: tls.RequireAndVerifyClientCert,
		},
	}

	serveErrCh := make(chan error, 1)

	go func() {
		serveErrCh <- server.ServeTLS(listener, certFile, keyFile)
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
