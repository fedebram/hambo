package main

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/fedebram/hambo/client"
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
			if !cmd.Flags().Changed("name") {
				hostname, err := os.Hostname()
				if err != nil {
					return fmt.Errorf("get hostname: %w", err)
				}
				name = hostname
			}
			if strings.TrimSpace(name) == "" {
				return errors.New("--name must not be empty")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			dataDir, err := cmd.Flags().GetString("data-dir")
			if err != nil {
				return err
			}

			parsedServerURL, err := client.ParseServerURL(serverURL)
			if err != nil {
				return fmt.Errorf("--server: %w", err)
			}

			result, err := worker.Join(cmd.Context(), worker.JoinConfig{
				ServerURL: parsedServerURL,
				Token:     joinToken,
				Name:      name,
				DataDir:   dataDir,
			})
			if err != nil {
				return err
			}

			if result.AlreadyConfigured {
				message := fmt.Sprintf("Worker already configured as node %s", result.NodeID)
				if joinToken != "" {
					message += "; supplied token was not used"
				}
				_, err = fmt.Fprintln(cmd.OutOrStdout(), message)
				return err
			}

			_, err = fmt.Fprintf(cmd.OutOrStdout(),
				"Worker joined as node %s\n", result.NodeID,
			)
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
		"Node join token (required for new enrollment)",
	)
	cmd.Flags().StringVar(
		&name,
		"name",
		"",
		"Node name",
	)

	return cmd
}
