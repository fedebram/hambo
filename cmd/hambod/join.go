package main

import (
	"github.com/fedebram/hambo/internal/worker"
	"github.com/spf13/cobra"
)

func joinCmd() *cobra.Command {
	var dataDir string
	var server string
	var token string

	cmd := &cobra.Command{
		Use:   "join",
		Short: "Join a server as a worker, with a join token",
		RunE: func(cmd *cobra.Command, args []string) error {
			return worker.Join(dataDir, server, token)
		},
	}
	cmd.Flags().StringVar(&dataDir, "data-dir", "./data", "where to store the worker's files")
	cmd.Flags().StringVar(&server, "server", "127.0.0.1:6443", "the server's address")
	cmd.Flags().StringVar(&token, "token", "", "the join token, from: hambo token create")
	cmd.MarkFlagRequired("token")
	return cmd
}
