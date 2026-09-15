package main

import (
	"context"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/docker/go-units"
	"github.com/fedebram/hambo/api"
	"github.com/fedebram/hambo/cli"
)

func newListImagesCommand(client imageClient) *cli.Command {
	command := cli.NewCommand("list", "List images")
	command.ValidateArgs = cli.NoArgs
	command.Run = func(
		ctx context.Context,
		args []string,
		out, errOut io.Writer,
	) error {
		images, err := client.ListImages(ctx)
		if err != nil {
			return fmt.Errorf("list images: %w", err)
		}

		return writeImageList(out, images)
	}
	return command
}

func writeImageList(out io.Writer, images []api.ImageSummary) error {
	w := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)
	if _, err := fmt.Fprintln(w, "REPOSITORY\tTAG\tDIGEST\tSIZE"); err != nil {
		return fmt.Errorf("write image list header: %w", err)
	}

	for _, image := range images {
		if _, err := fmt.Fprintf(
			w,
			"%s\t%s\t%s\t%s\n",
			displayImageField(image.Repository),
			displayImageField(image.Tag),
			shortImageDigest(image.Digest),
			units.HumanSize(float64(image.SizeBytes)),
		); err != nil {
			return fmt.Errorf("write image list: %w", err)
		}
	}

	if err := w.Flush(); err != nil {
		return fmt.Errorf("flush image list: %w", err)
	}
	return nil
}

func shortImageDigest(value string) string {
	_, encoded, found := strings.Cut(value, ":")
	if !found {
		encoded = value
	}
	if len(encoded) > shortImageDigestLength {
		return encoded[:shortImageDigestLength]
	}
	return encoded
}

func displayImageField(value string) string {
	if value == "" {
		return "<none>"
	}
	return value
}
