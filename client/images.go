package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/fedebram/hambo/api"
)

type PullImageProgressFunc func(api.ImagePullEvent) error

func (c *Client) ListImages(ctx context.Context) ([]api.ImageSummary, error) {
	var response api.ListImagesResponse
	if err := c.do(ctx, http.MethodGet, "images", nil, &response); err != nil {
		return nil, err
	}

	return response.Images, nil
}

func (c *Client) DeleteImage(ctx context.Context, selector string) (api.DeleteImageResponse, error) {
	input := api.DeleteImageRequest{Selector: selector}

	var response api.DeleteImageResponse
	if err := c.do(ctx, http.MethodDelete, "images", input, &response); err != nil {
		return api.DeleteImageResponse{}, err
	}
	return response, nil
}

func (c *Client) PullImage(ctx context.Context, reference string, report PullImageProgressFunc) (string, error) {
	input := api.PullImageRequest{Reference: reference}
	req, err := c.newRequest(ctx, http.MethodPost, "images/pull", input)
	if err != nil {
		return "", err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("send pull image request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", readResponseError(ctx, resp)
	}

	decoder := json.NewDecoder(resp.Body)
	for {
		var event api.ImagePullEvent
		if err := decoder.Decode(&event); err != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return "", ctxErr
			}
			if errors.Is(err, io.EOF) {
				return "", errors.New("image pull stream ended without a result")
			}
			return "", fmt.Errorf("decode image pull event: %w", err)
		}

		switch event.Type {
		case "progress":
			if report != nil {
				if err := report(event); err != nil {
					return "", fmt.Errorf("report image pull progress: %w", err)
				}
			}
		case "completed":
			if event.Digest == "" {
				// defensive, server side this is not possible
				return "", errors.New("image pull completed without a digest")
			}
			if err := drainResponseBody(ctx, resp.Body); err != nil {
				return "", err
			}
			return event.Digest, nil
		case "error":
			// the response status code here is 200... but it is fine to register that anyway.
			responseErr := newResponseError(resp.StatusCode, event.Error)
			if err := drainResponseBody(ctx, resp.Body); err != nil {
				return "", errors.Join(responseErr, err)
			}
			return "", responseErr
		default:
			return "", fmt.Errorf("unknown image pull event type %q", event.Type)
		}
	}
}
