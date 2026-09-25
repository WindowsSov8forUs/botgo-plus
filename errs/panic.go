package errs

import (
	"errors"
	"fmt"
	"runtime/debug"

	"github.com/WindowsSov8forUs/botgo-plus/log"
)

// PanicError retains the recovered value for explicit inspection, not default formatting.
// Stack contains at most 32 KiB of the current goroutine's diagnostic stack.
type PanicError struct {
	Value any
	Stack []byte
}

func NewPanicError(value any) *PanicError {
	stack := debug.Stack()
	if len(stack) > 32*1024 {
		stack = stack[:32*1024]
	}
	return &PanicError{Value: value, Stack: append([]byte(nil), stack...)}
}
func (e *PanicError) Error() string {
	if e == nil {
		return ""
	}
	text := fmt.Sprintf("QQ event handler panicked (%T)", e.Value)
	switch value := e.Value.(type) {
	case error:
		text += ": " + log.SafeError(value)
	case string:
		text += ": " + log.SafeError(errors.New(value))
	}
	return text
}
