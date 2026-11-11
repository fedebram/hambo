package main

import (
	"os"

	"github.com/spf13/cobra"
)

func main() {
	root := &cobra.Command{
		Use:          "hambod",
		Short:        "hambod is the hambo daemon: it runs a server or a worker",
		SilenceUsage: true,
	}
	root.AddCommand(initCmd(), serverCmd(), joinCmd())

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}
