package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/fedebram/hambo/api"
	"github.com/fedebram/hambo/errdefs"
)

type Error struct {
	StatusCode int
	Response   api.ErrorResponse
	cause      error
}

func (e *Error) Error() string {
	if e.Response.Message != "" {
		return e.Response.Message
	}
	return fmt.Sprintf("request failed with status code %d", e.StatusCode)
}

func (e *Error) Unwrap() error {
	return e.cause
}

func decodeResponseError(resp *http.Response) error {
	var response api.ErrorResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return fmt.Errorf(
			"decode response with status code %d: %w",
			resp.StatusCode,
			err,
		)
	}

	return &Error{
		StatusCode: resp.StatusCode,
		Response:   response,
		cause:      errorFromStatus(resp.StatusCode),
	}
}

func readResponseError(ctx context.Context, resp *http.Response) error {
	responseErr := decodeResponseError(resp)
	if ctxErr := ctx.Err(); ctxErr != nil {
		return ctxErr
	}
	if err := drainResponseBody(ctx, resp.Body); err != nil {
		return err
	}
	return responseErr
}

func errorFromStatus(statusCode int) error {
	switch statusCode {
	case http.StatusBadRequest,
		http.StatusUnsupportedMediaType,
		http.StatusUnprocessableEntity:
		return errdefs.ErrInvalidArgument
	case http.StatusUnauthorized:
		return errdefs.ErrUnauthenticated
	case http.StatusForbidden:
		return errdefs.ErrPermissionDenied
	case http.StatusNotFound:
		return errdefs.ErrNotFound
	case http.StatusMethodNotAllowed:
		return errdefs.ErrMethodNotAllowed
	case http.StatusConflict:
		return errdefs.ErrConflict
	case http.StatusInternalServerError:
		return errdefs.ErrInternal
	case http.StatusServiceUnavailable:
		return errdefs.ErrUnavailable
	default:
		return nil
	}
}
