package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newHealthCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "health",
		Short: "Check the Hambo server's health",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			apiClient, cleanup, err := newAPIClient(cmd)
			if err != nil {
				return err
			}
			defer cleanup()

			health, err := apiClient.Health(cmd.Context())
			if err != nil {
				return fmt.Errorf("check server health: %w", err)
			}

			_, err = fmt.Fprintln(cmd.OutOrStdout(), health.Status)
			return err
		},
	}
}
