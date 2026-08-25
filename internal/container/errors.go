package container

import "errors"

var (
	ErrNotFound            = errors.New("not found")
	ErrAlreadyExists       = errors.New("already exists")
	ErrOperationNotAllowed = errors.New("operation not allowed")
)
