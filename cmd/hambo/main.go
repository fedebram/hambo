package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/fedebram/hambo/internal/registry"
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
	root := &cobra.Command{
		Use:   "hambo",
		Short: "Manage containers and images",
	}

	root.AddCommand(&cobra.Command{
		Use:   "fetch <reference>",
		Short: "Fetch an OCI manifest",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			registryHost, repository, ref, err := registry.ParseImageReference(args[0])
			if err != nil {
				return err
			}
			httpClient := &http.Client{
				Timeout: 30 * time.Second,
			}

			manifest, err := registry.FetchManifest(httpClient, registryHost, repository, ref)
			if err != nil {
				return err
			}

			_, err = cmd.OutOrStdout().Write(manifest)
			return err
		},
	})

	return root
}
