package main

import (
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
