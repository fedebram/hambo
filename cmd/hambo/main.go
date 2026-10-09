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
	root := &cobra.Command{
		Use:   "hambo",
		Short: "Manage containers and images",
	}

	return root
}
