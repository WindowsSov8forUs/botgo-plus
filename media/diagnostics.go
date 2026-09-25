package media

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/WindowsSov8forUs/botgo-plus/errs"
	"github.com/WindowsSov8forUs/botgo-plus/log"
)

func putResponseError(response *http.Response, data []byte, cause error) error {
	var storage struct {
		Code      string
		Message   string
		RequestID string `xml:"RequestId"`
	}
	_ = xml.Unmarshal(data, &storage)
	text := fmt.Sprintf("signed PUT returned HTTP %d", response.StatusCode)
	if storage.Code != "" {
		text += ", storage code: " + log.SafeText(storage.Code)
	}
	if storage.Message != "" {
		text += ": " + log.SafeText(storage.Message)
	}
	requestID := response.Header.Get("X-Cos-Request-Id")
	if requestID == "" {
		requestID = storage.RequestID
	}
	if requestID != "" {
		text += ", request ID: " + log.SafeText(requestID)
	}
	return errs.NewResponseError("upload media part", response.StatusCode, response.Header, data, errors.Join(errors.New(text), cause))
}

func retryAfter(header http.Header) time.Duration {
	value := header.Get("Retry-After")
	if seconds, err := strconv.ParseInt(value, 10, 64); err == nil && seconds > 0 && seconds <= int64((1<<63-1)/time.Second) {
		return time.Duration(seconds) * time.Second
	}
	if when, err := http.ParseTime(value); err == nil && time.Until(when) > 0 {
		return time.Until(when)
	}
	return 0
}

func waitUploadRetry(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
