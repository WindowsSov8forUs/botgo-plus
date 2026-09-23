package errs

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

// APIError preserves the platform response. Body may contain user data and must not be logged by default.
type APIError struct {
	StatusCode int
	ErrorCode  int
	Message    string
	TraceID    string
	Header     http.Header
	Body       json.RawMessage
	RetryAfter time.Duration
}

func (e *APIError) Error() string {
	return fmt.Sprintf("QQ API: HTTP %d, code %d, trace %s", e.StatusCode, e.ErrorCode, e.TraceID)
}

// PendingError denotes acceptance for asynchronous processing, not successful delivery.
type PendingError struct {
	*APIError
	AuditID string
}

func (e *PendingError) Error() string { return "QQ operation pending: " + e.APIError.Error() }
func (e *PendingError) Unwrap() error { return e.APIError }

func responseCode(v json.RawMessage) (int, error) {
	s := string(bytes.TrimSpace(v))
	if s == "" || s == "null" {
		return 0, nil
	}
	if s[0] == '"' {
		var q string
		if err := json.Unmarshal(v, &q); err != nil {
			return 0, err
		}
		s = q
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, errors.New("QQ response error code must be an integer")
	}
	return n, nil
}

// CheckAPIResponse leaves successful objects, arrays and unknown fields intact.
func CheckAPIResponse(status int, header http.Header, body []byte) error {
	var fields map[string]json.RawMessage
	trimmed := bytes.TrimSpace(body)
	success := status >= 200 && status < 300
	if len(trimmed) > 0 && !json.Valid(trimmed) {
		if success {
			return json.Unmarshal(trimmed, new(json.RawMessage))
		}
	} else if len(trimmed) > 0 && trimmed[0] == '{' {
		_ = json.Unmarshal(trimmed, &fields)
	}
	code, parseErr := responseCode(fields["err_code"])
	if parseErr == nil && code == 0 {
		code, parseErr = responseCode(fields["code"])
	}
	if parseErr != nil && success {
		return parseErr
	}
	pending := success && (code == 304023 || code == 304024 || ((status == 201 || status == 202) && code == 0))
	if success && code == 0 && !pending {
		return nil
	}
	e := &APIError{StatusCode: status, ErrorCode: code, Header: header.Clone(), Body: append([]byte(nil), body...), TraceID: header.Get("X-Tps-trace-ID")}
	_ = json.Unmarshal(fields["message"], &e.Message)
	if e.TraceID == "" {
		_ = json.Unmarshal(fields["trace_id"], &e.TraceID)
	}
	if n, err := strconv.ParseInt(header.Get("Retry-After"), 10, 64); err == nil && n > 0 && n <= (1<<63-1)/int64(time.Second) {
		e.RetryAfter = time.Duration(n) * time.Second
	} else if when, err := http.ParseTime(header.Get("Retry-After")); err == nil {
		if wait := time.Until(when); wait > 0 {
			e.RetryAfter = wait
		}
	}
	if pending {
		var v struct {
			Data struct {
				MessageAudit struct {
					AuditID string `json:"audit_id"`
				} `json:"message_audit"`
			} `json:"data"`
		}
		_ = json.Unmarshal(body, &v)
		return &PendingError{APIError: e, AuditID: v.Data.MessageAudit.AuditID}
	}
	return e
}
