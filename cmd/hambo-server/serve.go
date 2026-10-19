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
	"github.com/fedebram/hambo/internal/pki"
	"github.com/fedebram/hambo/internal/store"
)

const (
	defaultAddress      = "127.0.0.1:8080"
	shutdownGracePeriod = 5 * time.Second
)

func newServeCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the Hambo server",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			dataDir, err := cmd.Flags().GetString("data-dir")
			if err != nil {
				return err
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
	return cmd
}

func run(ctx context.Context, dataDir string) (runErr error) {
	dbPath := filepath.Join(dataDir, "hambo.db")
	pkiDir := filepath.Join(dataDir, "pki")
	certFile := filepath.Join(pkiDir, "server.crt")
	keyFile := filepath.Join(pkiDir, "server.key")

	ca, err := pki.LoadCA(pkiDir)
	if err != nil {
		return fmt.Errorf("load CA: %w", err)
	}
	clientCAs := x509.NewCertPool()
	clientCAs.AddCert(ca.Certificate)

	if err := os.MkdirAll(filepath.Dir(dbPath), 0o700); err != nil {
		return fmt.Errorf("create db directory: %w", err)
	}

	db, err := store.Open(dbPath)
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			runErr = errors.Join(runErr, fmt.Errorf("close db: %w", err))
		}
	}()

	handler, err := api.NewServer(db, ca)
	if err != nil {
		return fmt.Errorf("create API server: %w", err)
	}

	listener, err := net.Listen("tcp", defaultAddress)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}

	return runServer(ctx, listener, handler, shutdownGracePeriod, certFile, keyFile, clientCAs)
}

func runServer(ctx context.Context, listener net.Listener, handler http.Handler, gracePeriod time.Duration, certFile, keyFile string, clientCAs *x509.CertPool) error {
	server := &http.Server{
		Handler: handler,
		TLSConfig: &tls.Config{
			ClientCAs: clientCAs,
			// this way we can have normal https routes and mtls routes.
			// we enforce via middleware.
			ClientAuth: tls.VerifyClientCertIfGiven,
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
