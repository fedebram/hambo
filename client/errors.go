package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/fedebram/hambo/api"
	"github.com/fedebram/hambo/errdefs"
)

type ResponseError struct {
	StatusCode int
	Code       string
	Message    string
	Fields     map[string]string
	cause      error
}

func (e *ResponseError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return fmt.Sprintf("request failed with status code %d", e.StatusCode)
}

func (e *ResponseError) Unwrap() error {
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

	return newResponseError(resp.StatusCode, response)
}

func newResponseError(statusCode int, response api.ErrorResponse) *ResponseError {
	return &ResponseError{
		StatusCode: statusCode,
		Code:       response.Code,
		Message:    response.Message,
		Fields:     response.Fields,
		cause:      errorFromCode(response.Code),
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

func errorFromCode(code string) error {
	switch code {
	case api.ErrorCodeNotFound:
		return errdefs.ErrNotFound
	case api.ErrorCodeOperationNotAllowed:
		return errdefs.ErrOperationNotAllowed
	case api.ErrorCodeValidationFailed,
		api.ErrorCodeInvalidJSON,
		api.ErrorCodeUnsupportedMediaType:
		return errdefs.ErrInvalidArgument
	case api.ErrorCodeInternal:
		return errdefs.ErrInternal
	default:
		return nil
	}
}
