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
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/fedebram/hambo/internal/api"
	"github.com/fedebram/hambo/internal/store"
)

const (
	defaultAddress      = "127.0.0.1:8080"
	shutdownGracePeriod = 5 * time.Second
)

func newServeCommand() *cobra.Command {
	var dataDir string

	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the Hambo server",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dataDir == "" {
				return errors.New("data directory cannot be empty")
			}
			slog.Info("starting hambo server", "address", "https://"+defaultAddress)

			if err := run(cmd.Context(), dataDir); err != nil {
				return err
			}

			slog.Info("hambo server stopped")
			slog.Info("bye bye")
			return nil
		},
	}

	cmd.Flags().StringVar(&dataDir, "data-dir", defaultDataDir, "Directory for persistent server data")
	return cmd
}

func run(ctx context.Context, dataDir string) (runErr error) {
	storePath := filepath.Join(dataDir, "hambo.db")
	pkiDir := filepath.Join(dataDir, "pki")
	caFile := filepath.Join(pkiDir, "ca.crt")
	certFile := filepath.Join(pkiDir, "server.crt")
	keyFile := filepath.Join(pkiDir, "server.key")

	caPEM, err := os.ReadFile(caFile)
	if err != nil {
		return fmt.Errorf("read client CA certificate: %w", err)
	}
	clientCAs := x509.NewCertPool()
	if !clientCAs.AppendCertsFromPEM(caPEM) {
		return errors.New("client CA file contains no valid PEM certificates")
	}

	if err := os.MkdirAll(filepath.Dir(storePath), 0o700); err != nil {
		return fmt.Errorf("create store directory: %w", err)
	}

	store, err := store.Open(storePath)
	if err != nil {
		return fmt.Errorf("open container store: %w", err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			runErr = errors.Join(runErr, fmt.Errorf("close container store: %w", err))
		}
	}()

	listener, err := net.Listen("tcp", defaultAddress)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}

	handler := api.NewServer()

	return runServer(ctx, listener, handler, shutdownGracePeriod, certFile, keyFile, clientCAs)
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
		slog.Info("shutting down hambo server")

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
