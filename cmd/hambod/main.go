package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func main() {
	root := &cobra.Command{
		Use:          "hambod",
		Short:        "hambod runs a hambo server",
		SilenceUsage: true,
	}
	root.AddCommand(initCmd(), serverCmd())

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

func serverCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "server",
		Short: "Start the server",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("server")
			return nil
		},
	}
}
