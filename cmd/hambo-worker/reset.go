package main

import (
	"fmt"

	"github.com/fedebram/hambo/internal/worker"
	"github.com/spf13/cobra"
)

func newResetCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "reset",
		Short: "Clear local worker enrollment",
		Long: "Clear local worker enrollment from the data directory. " +
			"Stop the worker before running reset. " +
			"The node registration on the server is left unchanged.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			dataDir, err := cmd.Flags().GetString("data-dir")
			if err != nil {
				return err
			}

			if err := worker.Reset(dataDir); err != nil {
				return err
			}

			_, err = fmt.Fprintln(cmd.OutOrStdout(), "Worker enrollment reset")
			return err
		},
	}
}
