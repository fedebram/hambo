package main

import (
	"fmt"
	"time"

	"github.com/fedebram/hambo/api"
	"github.com/spf13/cobra"
)

func newTokenCommand(cfg *cliConfig) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "token",
		Short: "Manage node join tokens",
	}

	cmd.AddCommand(newTokenCreateCommand(cfg))
	return cmd
}

func newTokenCreateCommand(cfg *cliConfig) *cobra.Command {
	var ttl time.Duration

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a node join token",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if ttl <= 0 || ttl > 24*time.Hour {
				return fmt.Errorf(
					"--ttl must be greater than zero and no more than 24h",
				)
			}

			apiClient, cleanup, err := newAPIClient(cfg.configPath)
			if err != nil {
				return err
			}
			defer cleanup()

			response, err := apiClient.CreateToken(
				cmd.Context(),
				api.CreateTokenRequest{TTL: ttl.String()},
			)
			if err != nil {
				return err
			}

			_, err = fmt.Fprintln(cmd.OutOrStdout(), response.Token)
			return err
		},
	}

	cmd.Flags().DurationVar(
		&ttl,
		"ttl",
		15*time.Minute,
		"Token lifespan (greater than zero, maximum 24h)",
	)
	return cmd
}
