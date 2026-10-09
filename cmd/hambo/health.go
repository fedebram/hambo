package main

import (
	"fmt"

	"github.com/fedebram/hambo/client"
	"github.com/spf13/cobra"
)

func newHealthCommand(cfg *cliConfig) *cobra.Command {
	return &cobra.Command{
		Use:   "health",
		Short: "Check the Hambo server's health",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			httpClient, err := client.NewMTLSClient(cfg.caFile, cfg.certFile, cfg.keyFile)
			if err != nil {
				return err
			}
			defer httpClient.CloseIdleConnections()

			apiClient, err := client.NewClient(cfg.serverURL, httpClient)
			if err != nil {
				return err
			}

			health, err := apiClient.Health(cmd.Context())
			if err != nil {
				return fmt.Errorf("check server health: %w", err)
			}

			_, err = fmt.Fprintln(cmd.OutOrStdout(), health.Status)
			return err
		},
	}
}
