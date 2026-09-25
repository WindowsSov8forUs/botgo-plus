package errs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"

	"github.com/WindowsSov8forUs/botgo-plus/log"
)

const diagnosticBodyLimit = 64 * 1024

// ResponseError marks a failure while reading or interpreting an upstream response.
// Body is a bounded diagnostic prefix, not a log field or a complete response guarantee.
type ResponseError struct {
	Operation  string
	StatusCode int
	TraceID    string
	Header     http.Header
	Body       []byte
	Truncated  bool
	Cause      error
}

func NewResponseError(operation string, status int, header http.Header, body []byte, cause error) *ResponseError {
	truncated := len(body) > diagnosticBodyLimit
	if truncated {
		body = body[:diagnosticBodyLimit]
	}
	e := &ResponseError{Operation: operation, StatusCode: status, TraceID: header.Get("X-Tps-trace-ID"), Header: header.Clone(), Body: append([]byte(nil), body...), Truncated: truncated, Cause: cause}
	if e.TraceID == "" {
		var value struct {
			TraceID string `json:"trace_id"`
		}
		if json.Unmarshal(body, &value) == nil {
			e.TraceID = value.TraceID
		}
	}
	return e
}

func (e *ResponseError) Error() string {
	if e == nil {
		return ""
	}
	text := e.Operation
	if e.StatusCode != 0 {
		text += fmt.Sprintf(" (upstream HTTP %d)", e.StatusCode)
	}
	if e.TraceID != "" {
		text += ", traceID: " + log.SafeText(e.TraceID)
	}
	if e.Cause != nil {
		text += ": " + log.SafeError(e.Cause)
	}
	return text
}
func (e *ResponseError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}
func (e *ResponseError) HTTPStatus() int {
	if e != nil {
		if errors.Is(e.Cause, context.Canceled) {
			return http.StatusServiceUnavailable
		}
		if errors.Is(e.Cause, context.DeadlineExceeded) {
			return http.StatusGatewayTimeout
		}
		var timeout net.Error
		if errors.As(e.Cause, &timeout) && timeout.Timeout() {
			return http.StatusGatewayTimeout
		}
	}
	return http.StatusBadGateway
}
func (e *ResponseError) ResponseHeaders() http.Header {
	if e == nil {
		return nil
	}
	h := e.Header.Clone()
	if h == nil {
		h = make(http.Header)
	}
	if e.TraceID != "" {
		h.Set("X-Tps-trace-ID", e.TraceID)
	}
	return h
}

func (e *APIError) ResponseHeaders() http.Header {
	if e == nil {
		return nil
	}
	h := e.Header.Clone()
	if h == nil {
		h = make(http.Header)
	}
	if e.TraceID != "" {
		h.Set("X-Tps-trace-ID", e.TraceID)
	}
	if e.ErrorCode != 0 {
		h.Set("X-QQ-Error-Code", strconv.Itoa(e.ErrorCode))
	}
	return h
}
