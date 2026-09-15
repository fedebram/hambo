package errdefs

import "errors"

var (
	ErrNotFound            = errors.New("not found")
	ErrInvalidArgument     = errors.New("invalid argument")
	ErrOperationNotAllowed = errors.New("operation not allowed")
	ErrInternal            = errors.New("internal error")
)
