package v1

import (
	"errors"
	"github.com/WindowsSov8forUs/botgo-plus/errs"
	"github.com/go-resty/resty/v2"
)

// responseError keeps response failures distinct from invalid caller parameters.
func responseError(resp *resty.Response, cause error) error {
	if cause == nil || resp == nil || resp.RawResponse == nil {
		return cause
	}
	var api *errs.APIError
	var upstream *errs.ResponseError
	if errors.As(cause, &api) || errors.As(cause, &upstream) {
		return cause
	}
	return errs.NewResponseError("process QQ API response", resp.StatusCode(), resp.Header(), resp.Body(), cause)
}

func (m *ResponseMeta) WrapError(operation string, cause error) error {
	if cause == nil || m == nil {
		return cause
	}
	var api *errs.APIError
	var upstream *errs.ResponseError
	if errors.As(cause, &api) || errors.As(cause, &upstream) {
		return cause
	}
	e := errs.NewResponseError(operation, m.StatusCode, m.Header, m.Raw, cause)
	e.TraceID = m.TraceID
	return e
}
