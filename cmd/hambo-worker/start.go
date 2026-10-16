package main

import (
	"github.com/fedebram/hambo/internal/worker"
	"github.com/spf13/cobra"
)

func newStartCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "start",
		Short: "Start the Hambo worker",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			dataDir, err := cmd.Flags().GetString("data-dir")
			if err != nil {
				return err
			}

			return worker.Start(cmd.Context(), dataDir)
		},
	}
}
