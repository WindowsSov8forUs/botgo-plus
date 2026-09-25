// Package v1 implements the native QQ API. The package name is retained for source compatibility.
package v1

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/WindowsSov8forUs/botgo-plus/constant"
	"github.com/WindowsSov8forUs/botgo-plus/errs"
	"github.com/WindowsSov8forUs/botgo-plus/log"
	"github.com/WindowsSov8forUs/botgo-plus/openapi"
	"github.com/WindowsSov8forUs/botgo-plus/token"
	"github.com/WindowsSov8forUs/botgo-plus/version"
	"github.com/go-resty/resty/v2"
	"golang.org/x/oauth2"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"time"
)

const MaxIdleConns = 128
const defaultMaxResponseBytes int64 = 16 * 1024 * 1024

type Client = openAPI
type ClientOption func(*clientConfig)
type clientConfig struct {
	baseURL          string
	timeout          time.Duration
	transport        http.RoundTripper
	maxResponseBytes int64
}

func WithBaseURL(s string) ClientOption { return func(c *clientConfig) { c.baseURL = s } }
func WithHTTPTransport(t http.RoundTripper) ClientOption {
	return func(c *clientConfig) { c.transport = t }
}
func WithRequestTimeout(t time.Duration) ClientOption { return func(c *clientConfig) { c.timeout = t } }
func WithMaxResponseBytes(n int64) ClientOption {
	return func(c *clientConfig) { c.maxResponseBytes = n }
}

type openAPI struct {
	appID       string
	tokenSource oauth2.TokenSource
	baseURL     string
	lastTraceID atomic.Value
	debug       atomic.Bool
	restyClient *resty.Client
}

func New(appID string, source oauth2.TokenSource, options ...ClientOption) (*Client, error) {
	if appID == "" || source == nil {
		return nil, errors.New("QQ app ID and token source are required")
	}
	if owner, ok := source.(interface{ GetAppID() string }); ok && owner.GetAppID() != "" && owner.GetAppID() != appID {
		return nil, errors.New("QQ token source belongs to a different application")
	}
	cfg := clientConfig{baseURL: constant.APIDomain, timeout: 15 * time.Second, maxResponseBytes: defaultMaxResponseBytes}
	for _, o := range options {
		o(&cfg)
	}
	u, err := url.Parse(cfg.baseURL)
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Scheme != "http" && u.Scheme != "https") {
		return nil, errors.New("invalid QQ API base URL")
	}
	if cfg.timeout < 0 || cfg.maxResponseBytes <= 0 {
		return nil, errors.New("timeout must be nonnegative and response limit positive")
	}
	if cfg.transport == nil {
		cfg.transport = createTransport(nil, MaxIdleConns)
	}
	api := &openAPI{appID: appID, tokenSource: source, baseURL: strings.TrimRight(u.String(), "/")}
	transport := &authorizedTransport{base: cfg.transport, source: source, origin: u, appID: appID, maxBytes: cfg.maxResponseBytes}
	hc := &http.Client{Transport: transport, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	api.restyClient = resty.NewWithClient(hc).SetTimeout(cfg.timeout).SetLogger(log.DefaultLogger).SetHeader("User-Agent", version.String()).SetHeader("Content-Type", "application/json")
	api.restyClient.OnAfterResponse(func(_ *resty.Client, resp *resty.Response) error {
		trace := resp.Header().Get(constant.HeaderTraceID)
		api.lastTraceID.Store(trace)
		if api.debug.Load() {
			message := fmt.Sprintf("[OPENAPI]%s %s returned %s in %s", resp.Request.Method, log.SafeText(resp.Request.RawRequest.URL.Path), resp.Status(), resp.Time())
			if trace != "" {
				message += ", traceID: " + log.SafeText(trace)
			}
			log.Debug(message)
		}
		if err := openapi.DoRespFilterChains(resp.Request.RawRequest, resp.RawResponse); err != nil {
			return err
		}
		if err := errs.CheckAPIResponse(resp.StatusCode(), resp.Header(), resp.Body()); err != nil {
			return err
		}
		if resp.Request.Result != nil && resp.StatusCode() != http.StatusNoContent {
			body := bytes.TrimSpace(resp.Body())
			if len(body) == 0 || bytes.Equal(body, []byte("null")) {
				return io.ErrUnexpectedEOF
			}
		}
		return nil
	})
	return api, nil
}
func Setup()                                   { openapi.Register(openapi.APIv1, &openAPI{}) }
func (o *openAPI) Version() openapi.APIVersion { return openapi.APIv1 }

// TraceID is race-safe but only describes the most recently completed request. Prefer ResponseMeta.
func (o *openAPI) TraceID() string {
	if s, ok := o.lastTraceID.Load().(string); ok {
		return s
	}
	return ""
}
func (o *openAPI) Setup(appID string, source oauth2.TokenSource, inSandbox bool) openapi.OpenAPI {
	var opts []ClientOption
	if inSandbox {
		opts = append(opts, WithBaseURL(constant.SandBoxAPIDomain))
	}
	api, err := New(appID, source, opts...)
	if err != nil {
		panic(err)
	}
	return api
}

// WithTimeout configures the HTTP client before use. Do not change it during requests.
// Zero disables the client timeout; caller context deadlines still apply.
func (o *openAPI) WithTimeout(d time.Duration) openapi.OpenAPI {
	if d < 0 {
		panic("QQ timeout must be nonnegative")
	}
	o.restyClient.SetTimeout(d)
	return o
}

// SetDebug enables metadata only, never secrets or payload bodies.
func (o *openAPI) SetDebug(b bool) openapi.OpenAPI { o.debug.Store(b); return o }
func (o *openAPI) GetAppID() string {
	if o == nil {
		return ""
	}
	return o.appID
}
func (o *openAPI) request(ctx context.Context) *resty.Request {
	return o.restyClient.R().SetContext(ctx)
}
func (o *openAPI) endpoint(path string) (string, error) {
	u, err := url.Parse(path)
	if err != nil {
		return "", err
	}
	if u.IsAbs() {
		return u.String(), nil
	}
	if u.Host != "" || u.Fragment != "" {
		return "", errors.New("invalid QQ API path")
	}
	return o.baseURL + "/" + strings.TrimLeft(path, "/"), nil
}
func (o *openAPI) Transport(ctx context.Context, method, path string, body interface{}) ([]byte, error) {
	endpoint, err := o.endpoint(path)
	if err != nil {
		return nil, err
	}
	resp, err := o.request(ctx).SetBody(body).Execute(strings.ToUpper(method), endpoint)
	if resp == nil {
		return nil, err
	}
	return append([]byte(nil), resp.Body()...), err
}

// ResponseMeta belongs to one request and preserves raw, including currently unknown, fields.
type ResponseMeta struct {
	StatusCode int
	TraceID    string
	Header     http.Header
	Raw        json.RawMessage
}

func (o *openAPI) Do(ctx context.Context, method, path string, body, out interface{}) (*ResponseMeta, error) {
	endpoint, err := o.endpoint(path)
	if err != nil {
		return nil, err
	}
	resp, err := o.request(ctx).SetBody(body).Execute(strings.ToUpper(method), endpoint)
	if resp == nil {
		return nil, err
	}
	meta := &ResponseMeta{StatusCode: resp.StatusCode(), TraceID: resp.Header().Get(constant.HeaderTraceID), Header: resp.Header().Clone(), Raw: append([]byte(nil), resp.Body()...)}
	if meta.TraceID == "" {
		var trace struct {
			TraceID string `json:"trace_id"`
		}
		if json.Unmarshal(meta.Raw, &trace) == nil {
			meta.TraceID = trace.TraceID
		}
	}
	if err != nil {
		return meta, meta.WrapError("process QQ API response", err)
	}
	if out != nil {
		body := bytes.TrimSpace(meta.Raw)
		if len(body) == 0 || bytes.Equal(body, []byte("null")) {
			err = io.ErrUnexpectedEOF
		} else if decodeErr := json.Unmarshal(body, out); decodeErr != nil {
			err = fmt.Errorf("decode QQ API response: %w", decodeErr)
		}
		if err != nil {
			return meta, meta.WrapError("process QQ API response", err)
		}
	}
	return meta, nil
}

type authorizedTransport struct {
	base     http.RoundTripper
	source   oauth2.TokenSource
	origin   *url.URL
	appID    string
	maxBytes int64
}

func (t *authorizedTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Body != nil {
		defer req.Body.Close()
	}
	if req.URL.Scheme != t.origin.Scheme || !strings.EqualFold(req.URL.Host, t.origin.Host) || req.URL.User != nil {
		return nil, errors.New("QQ credentials cannot be sent outside the configured API origin")
	}
	for attempt := 0; attempt < 2; attempt++ {
		tk, err := token.TokenContext(req.Context(), t.source)
		if err != nil {
			return nil, err
		}
		if tk == nil || tk.AccessToken == "" {
			return nil, errors.New("empty QQ access token")
		}
		clone := req.Clone(req.Context())
		if attempt > 0 && req.Body != nil {
			clone.Body, err = req.GetBody()
			if err != nil {
				return nil, err
			}
		}
		scheme := tk.TokenType
		if scheme == "" {
			scheme = token.TypeQQBot
		}
		clone.Header.Set("Authorization", scheme+" "+tk.AccessToken)
		clone.Header.Set("X-Union-Appid", t.appID)
		if err := openapi.DoReqFilterChains(clone, nil); err != nil {
			if clone.Body != nil {
				clone.Body.Close()
			}
			return nil, err
		}
		if clone.URL.Scheme != t.origin.Scheme || !strings.EqualFold(clone.URL.Host, t.origin.Host) || clone.URL.User != nil {
			if clone.Body != nil {
				clone.Body.Close()
			}
			return nil, errors.New("QQ request middleware changed the credential origin")
		}
		if err := clone.Context().Err(); err != nil {
			if clone.Body != nil {
				clone.Body.Close()
			}
			return nil, err
		}
		resp, err := t.base.RoundTrip(clone)
		if err != nil {
			return nil, err
		}
		data, readErr := io.ReadAll(io.LimitReader(resp.Body, t.maxBytes+1))
		resp.Body.Close()
		if readErr == nil && int64(len(data)) > t.maxBytes {
			readErr = errors.New("QQ response exceeds configured size limit")
		}
		if readErr != nil {
			return nil, errs.NewResponseError("read QQ API response", resp.StatusCode, resp.Header, data, readErr)
		}
		resp.Body = io.NopCloser(bytes.NewReader(data))
		var apiErr *errs.APIError
		classified := errs.CheckAPIResponse(resp.StatusCode, resp.Header, data)
		rejected := errors.As(classified, &apiErr) && (apiErr.StatusCode == 401 || apiErr.ErrorCode == errs.APICodeTokenExpireOrNotExist)
		invalidator, canInvalidate := t.source.(interface{ Invalidate(string) bool })
		if attempt == 0 && rejected && canInvalidate && (req.Body == nil || req.GetBody != nil) {
			resp.Body.Close()
			invalidator.Invalidate(tk.AccessToken)
			continue
		}
		return resp, nil
	}
	return nil, errors.New("QQ authentication retry exhausted")
}
func createTransport(localAddr net.Addr, idle int) *http.Transport {
	d := &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second, LocalAddr: localAddr}
	return &http.Transport{Proxy: http.ProxyFromEnvironment, DialContext: d.DialContext, ForceAttemptHTTP2: true, MaxIdleConns: idle, MaxIdleConnsPerHost: idle, IdleConnTimeout: 90 * time.Second, TLSHandshakeTimeout: 10 * time.Second, ExpectContinueTimeout: time.Second}
}
