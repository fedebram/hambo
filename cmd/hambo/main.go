package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
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

func newRootCommand() *cobra.Command {
	cfg := cliConfig{}
	root := &cobra.Command{
		Use:   "hambo",
		Short: "Manage containers and images",
	}

	root.PersistentFlags().StringVar(&cfg.serverURL, "server", "https://localhost:8080", "Hambo server URL")
	root.PersistentFlags().StringVar(&cfg.caFile, "ca-cert", "./certs/ca.crt", "Path to the trusted CA certificate in PEM format")
	root.AddCommand(newHealthCommand(&cfg))

	return root
}

type cliConfig struct {
	serverURL string
	caFile    string
}
