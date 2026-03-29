package webhook

import (
	"encoding/json"
	"io"
	"net/http"
	"os"

	"github.com/WindowsSov8forUs/botgo-plus/dto"
	"github.com/WindowsSov8forUs/botgo-plus/event"
	"github.com/WindowsSov8forUs/botgo-plus/interaction/signature"
	"github.com/WindowsSov8forUs/botgo-plus/log"
	"github.com/WindowsSov8forUs/botgo-plus/openapi"
)

type ack struct {
	Op   dto.OPCode `json:"op"`
	Data uint32     `json:"d"`
}

// GenHeartbeatACK builds heartbeat ack payload for HTTP callback gateway.
func GenHeartbeatACK(seq uint32) string {
	s, _ := json.Marshal(ack{Op: dto.WSHeartbeatAck, Data: seq})
	return string(s)
}

// GenDispatchACK builds dispatch ack payload for HTTP callback gateway.
func GenDispatchACK(success bool) string {
	var r uint32
	if !success {
		r = 1
	}
	s, _ := json.Marshal(ack{Op: dto.HTTPCallbackAck, Data: r})
	return string(s)
}

// DefaultGetSecretFunc gets webhook secret from environment.
var DefaultGetSecretFunc = func() string {
	return os.Getenv("QQBotSecret")
}

type HandlerOptions struct {
	// GetSecret gets the secret used for signature validation.
	// If nil, DefaultGetSecretFunc is used.
	GetSecret func(r *http.Request) string
	// ParsePayload handles parsed payload and returns response body.
	// If nil, default parsePayload is used.
	ParsePayload func(payload *dto.Payload, traceID string) (string, error)
}

// HTTPHandler keeps backward-compatible default behavior.
func HTTPHandler(w http.ResponseWriter, r *http.Request) {
	HTTPHandlerWithOptions(HandlerOptions{})(w, r)
}

// HTTPHandlerWithOptions handles callback request with configurable secret/parser.
func HTTPHandlerWithOptions(options HandlerOptions) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		body := make([]byte, r.ContentLength)
		if _, err := r.Body.Read(body); err != nil && err != io.EOF {
			log.Errorf("read http callback body error: %s", err)
			return
		}
		log.Debugf("http callback body: %v", string(body))

		secret := DefaultGetSecretFunc()
		if options.GetSecret != nil {
			secret = options.GetSecret(r)
		}
		traceID := r.Header.Get(openapi.TraceIDKey)
		if pass, err := signature.Verify(secret, r.Header, body); err != nil || !pass {
			log.Errorf("signature verify failed, err: %v, traceID: %s", err, traceID)
			return
		}

		payload := &dto.Payload{}
		if err := json.Unmarshal(body, payload); err != nil {
			log.Errorf("unmarshal http callback body error: %s, traceID: %s", err, traceID)
			return
		}
		payload.RawMessage = body

		parser := options.ParsePayload
		if parser == nil {
			parser = func(payload *dto.Payload, traceID string) (string, error) {
				return parsePayload(payload, traceID), nil
			}
		}
		result, err := parser(payload, traceID)
		if err != nil {
			log.Errorf("custom parse payload failed, err: %v, traceID: %s", err, traceID)
			return
		}
		if result != "" {
			if _, err := w.Write([]byte(result)); err != nil {
				log.Errorf("write http callback response error: %s, traceID: %s", err, traceID)
				return
			}
		}
	}
}

func parsePayload(payload *dto.Payload, traceID string) string {
	if payload.OPCode == dto.WSHeartbeat {
		return GenHeartbeatACK(uint32(payload.Data.(float64)))
	}
	if payload.OPCode == dto.DispatchEvent {
		if err := event.ParseAndHandle(payload); err != nil {
			log.Errorf("parseAndHandle failed, %v, traceID:%s, payload: %v", err, traceID, payload)
			return GenDispatchACK(false)
		}
		return GenDispatchACK(true)
	}
	return ""
}
