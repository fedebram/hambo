package main

import (
	"context"

	"github.com/fedebram/hambo/api"
	"github.com/fedebram/hambo/cli"
	hamboclient "github.com/fedebram/hambo/client"
)

const shortImageDigestLength = 12

type imageClient interface {
	ListImages(ctx context.Context) ([]api.ImageSummary, error)
	DeleteImage(ctx context.Context, selector string) (api.DeleteImageResponse, error)
	PullImage(
		ctx context.Context,
		reference string,
		report hamboclient.PullImageProgressFunc,
	) (string, error)
}

func newImageCommand(client imageClient) *cli.Command {
	command := cli.NewCommand("image", "Manage images")
	command.AddCommand(
		newListImagesCommand(client),
		newPullImageCommand(client),
		newRemoveImageCommand(client),
	)
	return command
}
