package cli

import (
	"fmt"
	"strings"
)

type UsageError struct {
	path []string
	err  error
}

func UsageErrorf(format string, a ...any) error {
	return &UsageError{
		err: fmt.Errorf(format, a...),
	}
}

func (e *UsageError) Error() string {
	return e.err.Error()
}

func (e *UsageError) Unwrap() error {
	return e.err
}

func (e *UsageError) CommandPath() string {
	return strings.Join(e.path, " ")
}
