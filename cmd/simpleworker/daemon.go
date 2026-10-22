package main

import "github.com/spf13/cobra"

func newDaemonCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "daemon",
		Short: "Run the worker daemon",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Println("Worker daemon started")
			<-cmd.Context().Done()
			cmd.Println("Worker daemon stopped")
		},
	}
}
