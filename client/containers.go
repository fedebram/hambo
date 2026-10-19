package client

import (
	"context"
	"net/http"

	"github.com/fedebram/hambo/api"
)

func (c *Client) CreateContainer(
	ctx context.Context,
	request api.CreateContainerRequest,
) (api.ContainerResponse, error) {
	var response api.ContainerResponse
	err := c.do(ctx, http.MethodPost, "containers", request, &response)
	return response, err
}
