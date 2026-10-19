package main

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
)

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

const defaultDataDir = "./data"

func newRootCommand() *cobra.Command {
	root := &cobra.Command{
		Use:   "hambo-worker",
		Short: "Manage containers",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			dataDir, err := cmd.Flags().GetString("data-dir")
			if err != nil {
				return err
			}
			if strings.TrimSpace(dataDir) == "" {
				return errors.New("--data-dir must not be empty")
			}
			return nil
		},
	}

	root.PersistentFlags().String(
		"data-dir",
		defaultDataDir,
		"Directory for persistent worker data",
	)

	root.AddCommand(newJoinCommand())
	root.AddCommand(newResetCommand())
	root.AddCommand(newStartCommand())

	return root
}
