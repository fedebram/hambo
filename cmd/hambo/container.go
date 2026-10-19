package main

import (
	"fmt"
	"strings"

	"github.com/fedebram/hambo/api"
	"github.com/spf13/cobra"
)

func newContainerCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "container",
		Short: "Manage containers",
	}

	cmd.AddCommand(newRunCommand())

	return cmd
}

func newRunCommand() *cobra.Command {
	var image string

	cmd := &cobra.Command{
		Use:   "run <name>",
		Short: "Run a container on the cluster",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(image) == "" {
				return fmt.Errorf("--image must not be empty")
			}

			apiClient, cleanup, err := newAPIClient(cmd)
			if err != nil {
				return err
			}
			defer cleanup()

			response, err := apiClient.CreateContainer(
				cmd.Context(),
				api.CreateContainerRequest{
					Name:  args[0],
					Image: image,
				},
			)
			if err != nil {
				return err
			}

			_, err = fmt.Fprintf(
				cmd.OutOrStdout(),
				"Container %s created (%s)\n",
				response.Name,
				response.State,
			)
			return err
		},
	}

	cmd.Flags().StringVar(
		&image,
		"image",
		"",
		"Container image",
	)

	return cmd
}
