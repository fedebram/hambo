package main

import (
	"context"
	"fmt"
	"io"

	"github.com/fedebram/hambo/cli"
)

func newRemoveImageCommand(client imageClient) *cli.Command {
	command := cli.NewCommand("rm", "Remove an image")
	command.ArgsUsage = "<selector>"
	command.ValidateArgs = cli.ExactArgs(1)

	command.Run = func(
		ctx context.Context,
		args []string,
		out, errOut io.Writer,
	) error {
		response, err := client.DeleteImage(ctx, args[0])
		if err != nil {
			return fmt.Errorf("remove image %q: %w", args[0], err)
		}

		if response.RemovedReference != "" {
			if _, err := fmt.Fprintln(out, "Removed reference:", response.RemovedReference); err != nil {
				return fmt.Errorf("write removed image reference: %w", err)
			}
		}
		if response.RemovedImage != "" {
			if _, err := fmt.Fprintln(out, "Deleted:", response.RemovedImage); err != nil {
				return fmt.Errorf("write removed image: %w", err)
			}
		}

		return nil
	}
	return command
}
