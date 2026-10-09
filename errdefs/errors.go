package errdefs

import "errors"

var (
	ErrInvalidArgument  = errors.New("invalid argument")
	ErrUnauthenticated  = errors.New("unauthenticated")
	ErrPermissionDenied = errors.New("permission denied")
	ErrNotFound         = errors.New("not found")
	ErrMethodNotAllowed = errors.New("method not allowed")
	ErrConflict         = errors.New("conflict")
	ErrInternal         = errors.New("internal error")
	ErrUnavailable      = errors.New("unavailable")
)
