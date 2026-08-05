package container

import "errors"

var (
	ErrNotFound            = errors.New("container not found")
	ErrAlreadyExists       = errors.New("container already exists")
	ErrOperationNotAllowed = errors.New("container operation not allowed")
)
