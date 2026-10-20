package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
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
	code := run(ctx)
	stop()
	os.Exit(code)
}

func run(ctx context.Context) int {
	rootCmd, err := newRootCommand()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	if err := rootCmd.ExecuteContext(ctx); err != nil {
		return 1
	}
	return 0
}

func newRootCommand() (*cobra.Command, error) {
	configPath := os.Getenv("HAMBO_CONFIG")
	if configPath == "" {
		configDir, err := os.UserConfigDir()
		if err != nil {
			return nil, fmt.Errorf("find user configuration directory: %w", err)
		}
		configPath = filepath.Join(configDir, "hambo", "config.yaml")
	}

	root := &cobra.Command{
		Use:   "hambo",
		Short: "Manage containers and images",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			path, err := cmd.Flags().GetString("config")
			if err != nil {
				return err
			}
			if strings.TrimSpace(path) == "" {
				return fmt.Errorf("--config must not be empty")
			}
			return nil
		},
	}

	root.PersistentFlags().String(
		"config",
		configPath,
		"Path to configuration file",
	)
	root.AddCommand(newHealthCommand())
	root.AddCommand(newTokenCommand())
	root.AddCommand(newContainerCommand())

	return root, nil
}
