package v1

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/WindowsSov8forUs/botgo-plus/dto"
	"github.com/WindowsSov8forUs/botgo-plus/errs"
	"golang.org/x/oauth2"
)

func staticSource() oauth2.TokenSource {
	return oauth2.StaticTokenSource(&oauth2.Token{AccessToken: "fixture", TokenType: "QQBot"})
}

func TestQQMessages(t *testing.T) {
	ctx := context.Background()
	message := &dto.MessageToCreate{Content: "text", MsgID: "incoming", MsgSeq: 3, MessageReference: &dto.MessageReference{MessageID: "REFIDX_quote=="}}
	for _, tc := range []struct {
		name, path string
		send       func(*Client) (*dto.Message, error)
	}{
		{"group", "/v2/groups/group/messages", func(c *Client) (*dto.Message, error) { return c.PostGroupMessage(ctx, "group", message) }},
		{"c2c", "/v2/users/user/messages", func(c *Client) (*dto.Message, error) { return c.PostC2CMessage(ctx, "user", message) }},
		{"channel", "/channels/channel/messages", func(c *Client) (*dto.Message, error) { return c.PostMessage(ctx, "channel", message) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var body dto.MessageToCreate
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Error(err)
				}
				if r.Method != "POST" || r.URL.Path != tc.path || r.Header.Get("Authorization") != "QQBot fixture" {
					t.Errorf("request=%s %s auth=%q", r.Method, r.URL.Path, r.Header.Get("Authorization"))
				}
				if body.Content != "text" || body.MsgID != "incoming" || body.MsgSeq != 3 || body.MessageReference == nil || body.MessageReference.MessageID != "REFIDX_quote==" {
					t.Errorf("message body=%+v", body)
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = io.WriteString(w, `{"id":"sent","ext_info":{"ref_idx":"REFIDX_sent=="}}`)
			}))
			defer server.Close()
			client, err := New("app", staticSource(), WithBaseURL(server.URL))
			if err != nil {
				t.Fatal(err)
			}
			result, err := tc.send(client)
			if err != nil {
				t.Fatal(err)
			}
			if result.ID != "sent" || result.ExtInfo == nil || result.ExtInfo.RefIdx != "REFIDX_sent==" {
				t.Fatalf("sent message=%+v", result)
			}
		})
	}
}

func TestQQGroupAPI(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Tps-trace-ID", "group-trace")
		switch r.Method + " " + r.URL.Path {
		case "GET /v2/groups/group/members":
			switch r.URL.Query().Get("cursor") {
			case "":
				_, _ = io.WriteString(w, `{"members":[{"member_openid":"one","member_role":"admin"}],"next_cursor":"next+/=="}`)
			case "next+/==":
				_, _ = io.WriteString(w, `{"members":[{"member_openid":"two"}],"next_cursor":""}`)
			default:
				t.Errorf("cursor=%q", r.URL.Query().Get("cursor"))
				w.WriteHeader(400)
			}
		case "POST /v2/groups/group/batch_remove_members":
			var body dto.QQGroupRemoveRequest
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Error(err)
			}
			if len(body.MemberOpenIDs) != 1 || body.MemberOpenIDs[0] != "one" {
				t.Errorf("remove=%+v", body)
			}
			_, _ = io.WriteString(w, `{"remove_members_result":"success","add_to_member_blacklist_fail_openids":["one"]}`)
		case "POST /v2/groups/group/restrict_chat_setting":
			var body dto.QQGroupMuteRequest
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Error(err)
			}
			if len(body.Members) != 1 || body.Members[0].Op != "del" || body.Members[0].MemberOpenID != "one" {
				t.Errorf("mute=%+v", body)
			}
			_, _ = io.WriteString(w, `{}`)
		default:
			t.Errorf("request=%s %s", r.Method, r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	client, err := New("app", staticSource(), WithBaseURL(server.URL))
	if err != nil {
		t.Fatal(err)
	}
	first, meta, err := client.GetQQGroupMembers(context.Background(), "group", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Members) != 1 || first.Members[0].MemberRole != "admin" || first.NextCursor != "next+/==" || meta.TraceID != "group-trace" {
		t.Fatalf("first page=%+v meta=%+v", first, meta)
	}
	second, _, err := client.GetQQGroupMembers(context.Background(), "group", first.NextCursor)
	if err != nil {
		t.Fatal(err)
	}
	if len(second.Members) != 1 || second.Members[0].MemberOpenID != "two" || second.NextCursor != "" {
		t.Fatalf("second page=%+v", second)
	}
	removed, _, err := client.RemoveQQGroupMembers(context.Background(), "group", &dto.QQGroupRemoveRequest{MemberOpenIDs: []string{"one"}})
	if err != nil {
		t.Fatal(err)
	}
	if removed.RemoveMembersResult != "success" || len(removed.BlacklistFailedOpenIDs) != 1 || removed.BlacklistFailedOpenIDs[0] != "one" {
		t.Fatalf("remove result=%+v", removed)
	}
	if _, err := client.SetQQGroupMemberMute(context.Background(), "group", &dto.QQGroupMuteRequest{Members: []dto.QQGroupMuteOperation{{Op: "del", MemberOpenID: "one"}}}); err != nil {
		t.Fatal(err)
	}
}

func TestStreamMediaAndInteraction(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]json.RawMessage
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.Method + " " + r.URL.Path {
		case "POST /v2/users/user/stream_messages":
			if string(body["index"]) != "0" || string(body["input_state"]) != "1" || string(body["content_raw"]) != `"text"` {
				t.Errorf("stream=%s", body)
			}
			_, _ = io.WriteString(w, `{"id":"stream","ext_info":{"ref_idx":"REFIDX_stream=="}}`)
		case "POST /v2/groups/group/files":
			if string(body["url"]) != `"https://example.invalid/file"` || string(body["file_type"]) != "4" || string(body["srv_send_msg"]) != "false" {
				t.Errorf("upload=%s", body)
			}
			_, _ = io.WriteString(w, `{"file_uuid":"uuid","file_info":"opaque!file-info","ttl":100}`)
		case "PUT /interactions/interaction":
			if string(body["code"]) != "0" {
				t.Errorf("interaction=%s", body)
			}
			w.WriteHeader(204)
		default:
			t.Errorf("request=%s %s", r.Method, r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	client, err := New("app", staticSource(), WithBaseURL(server.URL))
	if err != nil {
		t.Fatal(err)
	}
	message, _, err := client.PostC2CStreamMessage(context.Background(), "user", &dto.C2CStreamRequest{InputState: 1, ContentRaw: "text", MsgID: "incoming", MsgSeq: 1})
	if err != nil {
		t.Fatal(err)
	}
	if message.ID != "stream" || message.ExtInfo == nil || message.ExtInfo.RefIdx != "REFIDX_stream==" {
		t.Fatalf("stream result=%+v", message)
	}
	uploaded, _, err := client.UploadGroupFile(context.Background(), "group", &dto.MediaUploadRequest{URL: "https://example.invalid/file", FileType: 4, FileName: "fixture.txt"})
	if err != nil {
		t.Fatal(err)
	}
	if uploaded.FileUUID != "uuid" || uploaded.FileInfo != "opaque!file-info" || uploaded.TTL != 100 {
		t.Fatalf("upload result=%+v", uploaded)
	}
	meta, err := client.AcknowledgeInteraction(context.Background(), "interaction", 0)
	if err != nil {
		t.Fatal(err)
	}
	if meta.StatusCode != 204 {
		t.Fatalf("interaction status=%d", meta.StatusCode)
	}
}

type rotatingSource struct {
	mu      sync.Mutex
	current string
}

func (s *rotatingSource) Token() (*oauth2.Token, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return &oauth2.Token{AccessToken: s.current, TokenType: "QQBot"}, nil
}
func (s *rotatingSource) Invalidate(old string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.current != old {
		return false
	}
	s.current = "new"
	return true
}

func TestAuthenticationRefresh(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body dto.MessageToCreate
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		if body.MsgID != "incoming" || body.MsgSeq != 3 || body.Content != "text" {
			t.Errorf("reply=%+v", body)
		}
		w.Header().Set("Content-Type", "application/json")
		if calls.Add(1) == 1 {
			if r.Header.Get("Authorization") != "QQBot old" {
				t.Errorf("initial auth=%s", r.Header.Get("Authorization"))
			}
			w.WriteHeader(401)
			_, _ = io.WriteString(w, `{"code":11244}`)
		} else {
			if r.Header.Get("Authorization") != "QQBot new" {
				t.Errorf("refreshed auth=%s", r.Header.Get("Authorization"))
			}
			_, _ = io.WriteString(w, `{"id":"sent"}`)
		}
	}))
	defer server.Close()
	client, err := New("app", &rotatingSource{current: "old"}, WithBaseURL(server.URL))
	if err != nil {
		t.Fatal(err)
	}
	message, err := client.PostGroupMessage(context.Background(), "group", &dto.MessageToCreate{Content: "text", MsgID: "incoming", MsgSeq: 3})
	if err != nil {
		t.Fatal(err)
	}
	if message.ID != "sent" || calls.Load() != 2 {
		t.Fatalf("message=%+v authentication requests=%d", message, calls.Load())
	}
}

func TestAPIResponses(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		body   string
		code   int
		audit  string
	}{
		{"array", 200, `[{"id":"one"}]`, 0, ""},
		{"permission", 403, `{"err_code":11253}`, 11253, ""},
		{"reply-audit", 200, `{"err_code":304024,"data":{"message_audit":{"audit_id":"audit"}}}`, 304024, "audit"},
		{"push-audit", 202, `{"code":304023,"data":{"message_audit":{"audit_id":"audit"}}}`, 304023, "audit"},
		{"failed-async", 202, `{"err_code":11253}`, 11253, ""},
		{"rate-limit", 429, `{"code":"12"}`, 12, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("X-Tps-trace-ID", "response-trace")
				w.WriteHeader(tc.status)
				_, _ = io.WriteString(w, tc.body)
			}))
			defer server.Close()
			client, err := New("app", staticSource(), WithBaseURL(server.URL))
			if err != nil {
				t.Fatal(err)
			}
			body, err := client.Transport(context.Background(), "GET", "/fixture", nil)
			if string(body) != tc.body {
				t.Fatalf("response=%s, want %s", body, tc.body)
			}
			if tc.code == 0 {
				if err != nil {
					t.Fatal(err)
				}
				var values []dto.Message
				if err := json.Unmarshal(body, &values); err != nil {
					t.Fatal(err)
				}
				if len(values) != 1 || values[0].ID != "one" {
					t.Fatalf("array=%+v", values)
				}
				return
			}
			var apiError *errs.APIError
			if !errors.As(err, &apiError) || apiError.ErrorCode != tc.code || apiError.StatusCode != tc.status || apiError.TraceID != "response-trace" {
				t.Fatalf("platform error=%T %v", err, err)
			}
			var pending *errs.PendingError
			if tc.audit != "" {
				if !errors.As(err, &pending) || pending.AuditID != tc.audit {
					t.Fatalf("audit result=%v", err)
				}
			} else if _, ok := err.(*errs.APIError); !ok {
				t.Fatalf("platform failure type=%T", err)
			}
		})
	}
}
