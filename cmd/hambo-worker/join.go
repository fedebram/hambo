package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/fedebram/hambo/internal/worker"
	"github.com/spf13/cobra"
)

func newJoinCommand() *cobra.Command {

	var serverURL string
	var joinToken string
	var name string
	cmd := &cobra.Command{
		Use:   "join",
		Short: "Join a Hambo cluster",
		Args:  cobra.NoArgs,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if joinToken == "" {
				return errors.New("--token is required")
			}
			if name == "" {
				hostname, err := os.Hostname()
				if err != nil {
					return fmt.Errorf("get hostname: %w", err)
				}
				name = hostname
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			dataDir, err := cmd.Flags().GetString("data-dir")
			if err != nil {
				return err
			}

			if err := worker.Join(cmd.Context(), worker.JoinConfig{
				ServerURL: serverURL,
				Token:     joinToken,
				Name:      name,
				DataDir:   dataDir,
			}); err != nil {
				return err
			}

			_, err = fmt.Fprintln(cmd.OutOrStdout(), "Worker joined the cluster")
			return err
		},
	}

	cmd.Flags().StringVar(
		&serverURL,
		"server",
		"https://localhost:8080",
		"Hambo server URL",
	)
	cmd.Flags().StringVar(
		&joinToken,
		"token",
		"",
		"Node join token",
	)
	cmd.Flags().StringVar(
		&name,
		"name",
		"",
		"Node name",
	)

	return cmd
}
