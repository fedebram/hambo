package main

import "github.com/spf13/cobra"

func newDaemonCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "daemon",
		Short: "Run the worker daemon",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			docker, err := newDockerClient(cmd.Context())
			if err != nil {
				return err
			}
			defer docker.Close()

			cmd.Println("Worker daemon started")
			<-cmd.Context().Done()
			cmd.Println("Worker daemon stopped")
			return nil
		},
	}
}
