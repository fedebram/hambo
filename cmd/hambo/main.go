package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
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

	cfg := cliConfig{configPath: configPath}
	root := &cobra.Command{
		Use:   "hambo",
		Short: "Manage containers and images",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if cfg.configPath == "" {
				return fmt.Errorf("--config must not be empty")
			}
			return nil
		},
	}

	root.PersistentFlags().StringVar(
		&cfg.configPath,
		"config",
		cfg.configPath,
		"Path to configuration file",
	)
	root.AddCommand(newHealthCommand(&cfg))

	return root, nil
}

type cliConfig struct {
	configPath string
}
