package media

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/WindowsSov8forUs/botgo-plus/dto"
	"github.com/WindowsSov8forUs/botgo-plus/errs"
	"github.com/WindowsSov8forUs/botgo-plus/log"
	v1 "github.com/WindowsSov8forUs/botgo-plus/openapi/v1"
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

// Retry only the documented transient confirmation error, never a whole upload or message.
func (u *Uploader) confirmChunk(ctx context.Context, target Target, request *dto.UploadPartFinishRequest, config dto.UploadConfig) (*v1.ResponseMeta, error) {
	window, delay := u.retrySettings(config)
	ctx, cancel := context.WithTimeout(ctx, window)
	defer cancel()
	var meta *v1.ResponseMeta
	var last error
	for attempt := 0; attempt < u.config.MaxConfirmAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return meta, errors.Join(err, last)
		}
		if target.Scope == GroupScope {
			meta, last = u.api.FinishGroupUploadPart(ctx, target.OpenID, request)
		} else {
			meta, last = u.api.FinishC2CUploadPart(ctx, target.OpenID, request)
		}
		if last == nil {
			return meta, nil
		}
		var api *errs.APIError
		if !errors.As(last, &api) || api.ErrorCode != 40093001 || attempt+1 == u.config.MaxConfirmAttempts {
			return meta, last
		}
		wait := delay
		if api.RetryAfter > wait {
			wait = api.RetryAfter
		}
		if err := waitUploadRetry(ctx, wait); err != nil {
			return meta, errors.Join(err, last)
		}
	}
	return meta, last
}
