package rpc

import "fmt"

// Error is a handler error that is sent to the other side.
type Error struct {
	Msg string
}

func (e *Error) Error() string { return e.Msg }

// Errorf returns an Error with a formatted message.
func Errorf(format string, args ...any) error {
	return &Error{Msg: fmt.Sprintf(format, args...)}
}
