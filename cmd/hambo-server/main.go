package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
)

const defaultDataDir = "./data"

func main() {
	ctx, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)

	err := newRootCommand().ExecuteContext(ctx)
	stop()
	if err != nil {
		os.Exit(1)
	}
}

func newRootCommand() *cobra.Command {
	root := &cobra.Command{
		Use:   "hambo-server",
		Short: "Run and initialize the Hambo server",
	}

	root.AddCommand(newInitCommand())
	root.AddCommand(newServeCommand())
	return root
}
