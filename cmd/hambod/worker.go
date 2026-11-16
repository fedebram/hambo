package main

import (
	"github.com/fedebram/hambo/internal/worker"
	"github.com/spf13/cobra"
)

func workerCmd() *cobra.Command {
	var dataDir string

	cmd := &cobra.Command{
		Use:   "worker",
		Short: "Start the worker",
		RunE: func(cmd *cobra.Command, args []string) error {
			return worker.Run(dataDir)
		},
	}
	cmd.Flags().StringVar(&dataDir, "data-dir", "./data", "where the worker's files are")
	return cmd
}
