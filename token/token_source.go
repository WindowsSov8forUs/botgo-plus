// Package token implements QQ access-token acquisition and concurrency-safe invalidation.
package token

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/WindowsSov8forUs/botgo-plus/constant"
	"github.com/WindowsSov8forUs/botgo-plus/errs"
	"github.com/WindowsSov8forUs/botgo-plus/log"
	"golang.org/x/oauth2"
	"golang.org/x/sync/singleflight"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	TypeBearer = "Bearer"
	TypeQQBot  = "QQBot"
)

type QQBotCredentials struct {
	AppID     string `yaml:"appid"`
	AppSecret string `yaml:"secret"`
}
type qqBotTokenReq struct {
	AppID        string `json:"appId"`
	ClientSecret string `json:"clientSecret"`
}
type qqBotTokenRsp struct {
	Code        int    `json:"code"`
	Message     string `json:"message"`
	AccessToken string `json:"access_token"`
	ExpiresIn   int64  `json:"expires_in"`
}

func (r *qqBotTokenRsp) UnmarshalJSON(data []byte) error {
	var raw struct {
		Code        int             `json:"code"`
		Message     string          `json:"message"`
		AccessToken string          `json:"access_token"`
		ExpiresIn   json.RawMessage `json:"expires_in"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	*r = qqBotTokenRsp{Code: raw.Code, Message: raw.Message, AccessToken: raw.AccessToken}
	if len(raw.ExpiresIn) == 0 || string(raw.ExpiresIn) == "null" {
		return nil
	}
	value := string(raw.ExpiresIn)
	if raw.ExpiresIn[0] == '"' {
		if err := json.Unmarshal(raw.ExpiresIn, &value); err != nil {
			return err
		}
	}
	n, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid expires_in: %w", err)
	}
	r.ExpiresIn = n
	return nil
}

type Option func(*QQBotTokenSource)

func WithEndpoint(endpoint string) Option { return func(s *QQBotTokenSource) { s.endpoint = endpoint } }
func WithHTTPClient(client *http.Client) Option {
	return func(s *QQBotTokenSource) { s.client = client }
}
func WithRequestTimeout(timeout time.Duration) Option {
	return func(s *QQBotTokenSource) { s.timeout = timeout }
}

// QQBotTokenSource caches immutable token copies. Construct one source per QQ application.
type QQBotTokenSource struct {
	credentials QQBotCredentials
	mu          sync.Mutex
	cached      *oauth2.Token
	sg          singleflight.Group
	endpoint    string
	client      *http.Client
	timeout     time.Duration
}

// NewQQBotTokenSource creates a source with optional endpoint, HTTP client and timeout configuration.
// It performs no network requests until a token is requested. The returned source implements
// oauth2.TokenSource and exposes TokenContext and Invalidate directly.
func NewQQBotTokenSource(credentials *QQBotCredentials, options ...Option) *QQBotTokenSource {
	s := &QQBotTokenSource{endpoint: getTokenURL(), client: &http.Client{}, timeout: 10 * time.Second}
	if credentials != nil {
		s.credentials = *credentials
	}
	for _, o := range options {
		o(s)
	}
	if s.client == nil {
		s.client = &http.Client{}
	}
	c := *s.client
	// Never redirect the application secret.
	c.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	s.client = &c
	if s.timeout <= 0 {
		s.timeout = 10 * time.Second
	}
	return s
}
func (s *QQBotTokenSource) cachedToken() *oauth2.Token {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.cached.Valid() {
		return nil
	}
	t := *s.cached
	return &t
}
func (s *QQBotTokenSource) Token() (*oauth2.Token, error) {
	return s.TokenContext(context.Background())
}
func (s *QQBotTokenSource) TokenContext(ctx context.Context) (*oauth2.Token, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if t := s.cachedToken(); t != nil {
		return t, nil
	}
	ch := s.sg.DoChan("access-token", func() (interface{}, error) {
		if t := s.cachedToken(); t != nil {
			return t, nil
		}
		// A canceled waiter does not cancel the bounded refresh shared with other callers.
		shared, cancel := context.WithTimeout(context.Background(), s.timeout)
		defer cancel()
		t, err := s.retrieve(shared)
		if err != nil {
			return nil, err
		}
		s.mu.Lock()
		s.cached = t
		s.mu.Unlock()
		return t, nil
	})
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case r := <-ch:
		if r.Err != nil {
			return nil, r.Err
		}
		t := *(r.Val.(*oauth2.Token))
		return &t, nil
	}
}

// Invalidate never clears a newer token when an older in-flight request is rejected.
func (s *QQBotTokenSource) Invalidate(rejected string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cached != nil && s.cached.AccessToken == rejected {
		s.cached = nil
		return true
	}
	return false
}
func (s *QQBotTokenSource) retrieve(ctx context.Context) (*oauth2.Token, error) {
	if s.credentials.AppID == "" || s.credentials.AppSecret == "" {
		return nil, errors.New("QQ app ID and secret are required")
	}
	u, err := url.Parse(s.endpoint)
	if err != nil || u.Host == "" || u.User != nil || (u.Scheme != "https" && u.Scheme != "http") {
		return nil, errors.New("invalid QQ token endpoint")
	}
	body, _ := json.Marshal(qqBotTokenReq{AppID: s.credentials.AppID, ClientSecret: s.credentials.AppSecret})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1024*1024+1))
	if err != nil {
		return nil, err
	}
	if len(data) > 1024*1024 {
		return nil, errors.New("QQ token response too large")
	}
	if err := errs.CheckAPIResponse(resp.StatusCode, resp.Header, data); err != nil {
		return nil, err
	}
	var result qqBotTokenRsp
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("decode token response: %w", err)
	}
	if result.AccessToken == "" || result.ExpiresIn <= 0 || result.ExpiresIn > int64(time.Duration(1<<63-1)/time.Second) {
		return nil, errors.New("invalid token or expiry in QQ response")
	}
	return &oauth2.Token{AccessToken: result.AccessToken, TokenType: TypeQQBot, ExpiresIn: result.ExpiresIn, Expiry: time.Now().Add(time.Duration(result.ExpiresIn) * time.Second)}, nil
}
func (s *QQBotTokenSource) GetAppID() string {
	if s == nil {
		return ""
	}
	return s.credentials.AppID
}
func TokenContext(ctx context.Context, source oauth2.TokenSource) (*oauth2.Token, error) {
	if source == nil {
		return nil, errors.New("nil token source")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if s, ok := source.(interface {
		TokenContext(context.Context) (*oauth2.Token, error)
	}); ok {
		return s.TokenContext(ctx)
	}
	return source.Token()
}

// StartRefreshAccessToken retains upstream's 9-9.5s lead and one-second failure retry.
// Schedule from the token's remaining lifetime, not its original expires_in.
func StartRefreshAccessToken(ctx context.Context, source oauth2.TokenSource) error {
	tk, err := TokenContext(ctx, source)
	if err != nil {
		return err
	}
	if tk == nil {
		return errors.New("token source returned a nil token")
	}
	go func() {
		for {
			wait := time.Second
			if tk != nil {
				if tk.Expiry.IsZero() {
					return // oauth2 treats a zero expiry as non-expiring.
				}
				// oauth2.Token.Valid uses a 10s margin; this timer runs just after
				// that cache threshold. Package-level rand is safe across sources.
				wait = time.Until(tk.Expiry) - 9*time.Second - time.Duration(rand.Int63n(500))*time.Millisecond
				if wait < time.Second {
					wait = time.Second
				}
			}
			timer := time.NewTimer(wait)
			select {
			case <-ctx.Done():
				timer.Stop()
				return
			case <-timer.C:
			}
			tk, err = TokenContext(ctx, source)
			if err != nil {
				tk = nil
				log.Warnf("QQ token refresh failed; retrying in one second")
			}
		}
	}()
	return nil
}
func getTokenURL() string {
	return strings.TrimRight(constant.TokenDomain, "/") + "/app/getAppAccessToken"
}
