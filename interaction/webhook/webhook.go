// Package webhook receives and verifies native QQ callbacks.
package webhook

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/WindowsSov8forUs/botgo-plus/dto"
	"github.com/WindowsSov8forUs/botgo-plus/errs"
	"github.com/WindowsSov8forUs/botgo-plus/event"
	"github.com/WindowsSov8forUs/botgo-plus/interaction/signature"
	"github.com/WindowsSov8forUs/botgo-plus/log"
	"github.com/WindowsSov8forUs/botgo-plus/token"
	"io"
	"net/http"
	"os"
)

const DefaultMaxBodyBytes int64 = 4 * 1024 * 1024

type ack struct {
	Op   dto.OPCode `json:"op"`
	Data uint32     `json:"d"`
}

func GenHeartbeatACK(seq uint32) string {
	b, _ := json.Marshal(ack{Op: dto.WSHeartbeatAck, Data: seq})
	return string(b)
}
func GenDispatchACK(success bool) string {
	var d uint32
	if !success {
		d = 1
	}
	b, _ := json.Marshal(ack{Op: dto.HTTPCallbackAck, Data: d})
	return string(b)
}

// Deprecated: pass explicit credentials to NewHandler.
var DefaultGetSecretFunc = func() string { return os.Getenv("QQBotSecret") }

type EventHandler func(context.Context, *dto.WSPayload) error
type Option func(*Handler)

// WithEventHandler processes or durably enqueues an event before acknowledging acceptance.
func WithEventHandler(f EventHandler) Option { return func(h *Handler) { h.accept = f } }
func WithMaxBodyBytes(n int64) Option        { return func(h *Handler) { h.maxBytes = n } }

type Handler struct {
	credentials token.QQBotCredentials
	accept      EventHandler
	maxBytes    int64
}

func NewHandler(credentials *token.QQBotCredentials, options ...Option) (*Handler, error) {
	if credentials == nil || credentials.AppID == "" || credentials.AppSecret == "" {
		return nil, errors.New("QQ webhook credentials are required")
	}
	h := &Handler{credentials: *credentials, maxBytes: DefaultMaxBodyBytes, accept: func(_ context.Context, p *dto.WSPayload) error { return event.ParseAndHandle(p) }}
	for _, o := range options {
		o(h)
	}
	if h.maxBytes <= 0 || h.accept == nil {
		return nil, errors.New("invalid QQ webhook options")
	}
	return h, nil
}
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		http.Error(w, "method not allowed", 405)
		return
	}
	if r.Header.Get("X-Bot-Appid") != h.credentials.AppID {
		http.Error(w, "unknown QQ application", 401)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, h.maxBytes)
	defer r.Body.Close()
	body, err := io.ReadAll(r.Body)
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			http.Error(w, "payload too large", 413)
		} else {
			http.Error(w, "invalid body", 400)
		}
		return
	}
	var envelope struct {
		Op   *dto.OPCode     `json:"op"`
		Data json.RawMessage `json:"d"`
	}
	if json.Unmarshal(body, &envelope) != nil || envelope.Op == nil {
		http.Error(w, "invalid payload", 400)
		return
	}
	// A challenge is never dispatched as an unsigned business event.
	if *envelope.Op == dto.HTTPCallbackValidation {
		var challenge dto.WHValidationReq
		if json.Unmarshal(envelope.Data, &challenge) != nil || challenge.PlainToken == "" || challenge.EventTs == "" {
			http.Error(w, "invalid challenge", 400)
			return
		}
		result := GenValidationACK(&challenge, r.Header, h.credentials.AppSecret)
		if result == nil {
			http.Error(w, "cannot sign challenge", 500)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(result)
		return
	}
	valid, err := signature.Verify(h.credentials.AppSecret, r.Header, body)
	if err != nil || !valid {
		http.Error(w, "invalid signature", 401)
		return
	}
	if *envelope.Op == dto.WSHeartbeat {
		var seq uint32
		if json.Unmarshal(envelope.Data, &seq) != nil {
			http.Error(w, "invalid heartbeat", 400)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, GenHeartbeatACK(seq))
		return
	}
	if *envelope.Op != dto.WSDispatchEvent {
		http.Error(w, "unsupported opcode", 400)
		return
	}
	var payload dto.WSPayload
	if json.Unmarshal(body, &payload) != nil || payload.Type == "" {
		http.Error(w, "invalid event", 400)
		return
	}
	payload.RawMessage = append([]byte(nil), body...)
	payload.Session = &dto.Session{AppID: h.credentials.AppID}
	err = acceptSafely(r.Context(), h.accept, &payload)
	w.Header().Set("Content-Type", "application/json")
	if err != nil {
		w.WriteHeader(503)
		io.WriteString(w, GenDispatchACK(false))
		return
	}
	io.WriteString(w, GenDispatchACK(true))
}
func acceptSafely(ctx context.Context, f EventHandler, p *dto.WSPayload) (err error) {
	defer func() {
		if value := recover(); value != nil {
			panicErr := errs.NewPanicError(value)
			err = panicErr
			log.Errorf("%v", panicErr)
			log.Debugf("QQ callback panic stack:\n%s", panicErr.Stack)
		}
	}()
	return f(ctx, p)
}

// HTTPHandler is the legacy wrapper; prefer constructing one Handler per application.
func HTTPHandler(w http.ResponseWriter, r *http.Request, credentials *token.QQBotCredentials) {
	h, err := NewHandler(credentials)
	if err != nil {
		http.Error(w, "invalid webhook configuration", 500)
		return
	}
	h.ServeHTTP(w, r)
}
func GenValidationACK(req *dto.WHValidationReq, header http.Header, secret string) []byte {
	if req == nil || req.PlainToken == "" || req.EventTs == "" {
		return nil
	}
	h := header.Clone()
	h.Set(signature.HeaderTimestamp, req.EventTs)
	sig, err := signature.Generate(secret, h, []byte(req.PlainToken))
	if err != nil {
		return nil
	}
	b, err := json.Marshal(dto.WHValidationRsp{PlainToken: req.PlainToken, Signature: sig})
	if err != nil {
		return nil
	}
	return b
}
