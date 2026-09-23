package webhook

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/WindowsSov8forUs/botgo-plus/dto"
	"github.com/WindowsSov8forUs/botgo-plus/interaction/signature"
	"github.com/WindowsSov8forUs/botgo-plus/token"
)

// Public signature fixture retained from the upstream test suite.
func TestSignatureVector(t *testing.T) {
	const secret = "123456abcdef"
	const expected = "e949b5b94ef4103df903fb031d1d16e358db3db83e79e117edd404c8508be3ce8a76d7bad1bed353194c126a1a5915b4ad8b5288c1191cc53a12acffccd82004"
	body := []byte(`{"id":"ROBOT1.0_veoihSEXDc8Q.g-6eLpNIa11bH8MisOjn-m-LKxCPntMk6exUXgcWCGpVO7L2QKTNZzjZzFFDSbiOFcqAPWyVA!!","content":"哦一下","timestamp":"2024-10-15T16:33:15+08:00","author":{"id":"675860273","user_openid":"675860273"}}`)
	header := make(http.Header)
	header.Set(signature.HeaderTimestamp, "1728981195")
	got, err := signature.Generate(secret, header, body)
	if err != nil || got != expected {
		t.Fatalf("signature=%s: %v", got, err)
	}
	header.Set(signature.HeaderSig, expected)
	if valid, err := signature.Verify(secret, header, body); err != nil || !valid {
		t.Fatalf("signature verification=%v: %v", valid, err)
	}
}

func TestWebhook(t *testing.T) {
	t.Run("challenge", func(t *testing.T) {
		// Public challenge fixture from QQ's Webhook documentation.
		credentials := token.QQBotCredentials{AppID: "11111111", AppSecret: "DG5g3B4j9X2KOErG"}
		handler, err := NewHandler(&credentials)
		if err != nil {
			t.Fatal(err)
		}
		request := httptest.NewRequest("POST", "/", strings.NewReader(`{"d":{"plain_token":"Arq0D5A61EgUu4OxUvOp","event_ts":"1725442341"},"op":13}`))
		request.Header.Set("X-Bot-Appid", credentials.AppID)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		var value dto.WHValidationRsp
		if err := json.Unmarshal(response.Body.Bytes(), &value); err != nil {
			t.Fatal(err)
		}
		const expected = "87befc99c42c651b3aac0278e71ada338433ae26fcb24307bdc5ad38c1adc2d01bcfcadc0842edac85e85205028a1132afe09280305f13aa6909ffc2d652c706"
		if response.Code != 200 || value.PlainToken != "Arq0D5A61EgUu4OxUvOp" || value.Signature != expected {
			t.Fatalf("challenge status=%d payload=%+v", response.Code, value)
		}
	})
	for _, tc := range []struct {
		name, body, ack string
		handlerErr      error
		status          int
	}{
		{"event", `{"op":0,"t":"GROUP_MESSAGE_CREATE","id":"event","d":{"id":"message"}}`, `{"op":12,"d":0}`, nil, 200},
		{"processing-error", `{"op":0,"t":"GROUP_MESSAGE_CREATE","id":"event","d":{"id":"message"}}`, `{"op":12,"d":1}`, errors.New("fixture processing error"), 503},
		{"heartbeat", `{"op":1,"d":7}`, `{"op":11,"d":7}`, nil, 200},
	} {
		t.Run(tc.name, func(t *testing.T) {
			credentials := token.QQBotCredentials{AppID: "app", AppSecret: "fixture-secret"}
			received := ""
			handler, err := NewHandler(&credentials, WithEventHandler(func(_ context.Context, p *dto.WSPayload) error {
				received = p.EventID
				if p.Session.AppID != "app" || string(p.RawMessage) != tc.body {
					t.Errorf("callback payload=%+v", p)
				}
				return tc.handlerErr
			}))
			if err != nil {
				t.Fatal(err)
			}
			request := httptest.NewRequest("POST", "/", strings.NewReader(tc.body))
			request.Header.Set("X-Bot-Appid", "app")
			request.Header.Set(signature.HeaderTimestamp, "1725442341")
			sig, err := signature.Generate(credentials.AppSecret, request.Header, []byte(tc.body))
			if err != nil {
				t.Fatal(err)
			}
			request.Header.Set(signature.HeaderSig, sig)
			request.Body = io.NopCloser(iotest.OneByteReader(strings.NewReader(tc.body)))
			request.ContentLength = -1
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != tc.status || response.Body.String() != tc.ack {
				t.Fatalf("status=%d ack=%s, want %d %s", response.Code, response.Body.String(), tc.status, tc.ack)
			}
			if tc.name != "heartbeat" && received != "event" {
				t.Fatalf("received event=%q", received)
			}
		})
	}
}
