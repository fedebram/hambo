package client

import (
	"context"
	"net/http"

	"github.com/fedebram/hambo/api"
)

func (c *Client) CreateToken(ctx context.Context, request api.CreateTokenRequest) (api.CreateTokenResponse, error) {
	var response api.CreateTokenResponse
	err := c.do(ctx, http.MethodPost, "tokens", request, &response)
	return response, err
}
