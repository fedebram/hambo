package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/fedebram/hambo/client"
	"github.com/fedebram/hambo/internal/worker"
	"github.com/spf13/cobra"
)

const (
	defaultServerURL = "https://localhost:8080"
	defaultCAFile    = "./certs/ca.crt"
	defaultCertFile  = "./certs/worker.crt"
	defaultKeyFile   = "./certs/worker.key"
)

func main() {
	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	code := run(ctx)
	stop()
	os.Exit(code)
}

func run(ctx context.Context) int {
	rootCmd := newRootCommand()
	if err := rootCmd.ExecuteContext(ctx); err != nil {
		return 1
	}
	return 0
}

type workerConfig struct {
	serverURL string
	caFile    string
	certFile  string
	keyFile   string
}

// we use cobra here because it is just simple and works.
// it is possible to use go flags but I have already setup this for the hambo cli so fine like that.

func newRootCommand() *cobra.Command {
	cfg := workerConfig{}
	root := &cobra.Command{
		Use:   "hambo-worker",
		Short: "Run a Hambo worker",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWorker(cmd.Context(), cfg)
		},
	}
	root.Flags().StringVar(&cfg.serverURL, "server", defaultServerURL, "Hambo server URL")
	root.Flags().StringVar(&cfg.caFile, "ca-cert", defaultCAFile, "Path to the trusted CA certificate in PEM format")
	root.Flags().StringVar(&cfg.certFile, "client-cert", defaultCertFile, "Path to the worker client certificate in PEM format")
	root.Flags().StringVar(&cfg.keyFile, "client-key", defaultKeyFile, "Path to the worker private key in PEM format")
	return root
}

func runWorker(ctx context.Context, cfg workerConfig) error {
	slog.Info("starting hambo worker", "server", cfg.serverURL)

	httpClient, err := client.NewMTLSClient(cfg.caFile, cfg.certFile, cfg.keyFile)
	if err != nil {
		return err
	}
	defer httpClient.CloseIdleConnections()

	apiClient, err := client.NewClient(cfg.serverURL, httpClient)
	if err != nil {
		return err
	}

	w := worker.NewWorker(apiClient)
	if err := w.Run(ctx); err != nil {
		return err
	}

	slog.Info("hambo worker stopped")
	return nil
}
