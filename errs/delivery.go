package errs

import (
	"encoding/json"
	"fmt"

	"github.com/WindowsSov8forUs/botgo-plus/log"
)

// EventHandlerError retains an unacknowledged event without assuming that replay is safe.
// Raw contains the received payload and may include credentials; it is never part of Error().
type EventHandlerError struct {
	EventType string
	Sequence  uint32
	Raw       json.RawMessage
	Cause     error
}

func (e *EventHandlerError) Error() string {
	if e == nil {
		return ""
	}
	text := fmt.Sprintf("QQ event %q at sequence %d was not acknowledged; automatic replay stopped", e.EventType, e.Sequence)
	if e.Cause != nil {
		text += ": " + log.SafeError(e.Cause)
	}
	return text
}

func (e *EventHandlerError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}
