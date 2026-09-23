package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/WindowsSov8forUs/botgo-plus/dto"
	"github.com/WindowsSov8forUs/botgo-plus/event"
	wss "github.com/gorilla/websocket"
	"golang.org/x/oauth2"
)

func TestGatewayFrames(t *testing.T) {
	frames := make(chan []byte, 4)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := (&wss.Upgrader{}).Upgrade(w, r, nil)
		if err != nil {
			t.Error(err)
			return
		}
		defer conn.Close()
		for {
			_, body, err := conn.ReadMessage()
			if err != nil {
				return
			}
			frames <- body
		}
	}))
	defer server.Close()
	source := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: "fixture", TokenType: "QQBot"})
	client := (&Client{}).New(dto.Session{ID: "session", URL: "ws" + strings.TrimPrefix(server.URL, "http"), TokenSource: source, LastSeq: 7, Intent: dto.IntentGroupMessages, Shards: dto.ShardConfig{ShardCount: 1}}).(*Client)
	defer client.Close()
	if err := client.Connect(); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		send func() error
		op   dto.OPCode
	}{
		{"identify", client.Identify, dto.WSIdentity},
		{"resume", client.Resume, dto.WSResume},
		{"heartbeat", client.heartbeatTick, dto.WSHeartbeat},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.send(); err != nil {
				t.Fatal(err)
			}
			select {
			case body := <-frames:
				var payload struct {
					Op   dto.OPCode      `json:"op"`
					Data json.RawMessage `json:"d"`
				}
				if err := json.Unmarshal(body, &payload); err != nil {
					t.Fatal(err)
				}
				if payload.Op != tc.op {
					t.Fatalf("opcode=%d, want %d", payload.Op, tc.op)
				}
				switch tc.op {
				case dto.WSIdentity:
					var value dto.WSIdentityData
					if err := json.Unmarshal(payload.Data, &value); err != nil {
						t.Fatal(err)
					}
					if value.Token != "QQBot fixture" || value.Intents != dto.IntentGroupMessages || len(value.Shard) != 2 || value.Shard[0] != 0 || value.Shard[1] != 1 {
						t.Fatalf("identify=%+v", value)
					}
				case dto.WSResume:
					var value dto.WSResumeData
					if err := json.Unmarshal(payload.Data, &value); err != nil {
						t.Fatal(err)
					}
					if value.Token != "QQBot fixture" || value.SessionID != "session" || value.Seq != 7 {
						t.Fatalf("resume=%+v", value)
					}
				case dto.WSHeartbeat:
					if string(payload.Data) != "7" {
						t.Fatalf("heartbeat=%s", payload.Data)
					}
					client.isHandleBuildIn(&dto.WSPayload{WSPayloadBase: dto.WSPayloadBase{OPCode: dto.WSHeartbeatAck}})
					if err := client.heartbeatTick(); err != nil {
						t.Fatal(err)
					}
				}
			case <-time.After(2 * time.Second):
				t.Fatal("gateway frame timed out")
			}
		})
	}
}

func TestGatewayDispatch(t *testing.T) {
	previous := event.DefaultHandlers
	defer func() { event.DefaultHandlers = previous }()
	frames := []string{
		`{"op":0,"s":1,"t":"READY","d":{"session_id":"fresh","shard":[0,1],"user":{"id":"bot"}}}`,
		`{"op":0,"s":2,"t":"RESUMED","d":""}`,
		`{"op":0,"s":3,"t":"FUTURE_EVENT","id":"event-id","d":{"extra":"value"}}`,
	}
	for _, mode := range []string{"global", "callback", "dispatcher"} {
		t.Run(mode, func(t *testing.T) {
			received := 0
			record := func(route string, p *dto.WSPayload) {
				if route != mode || p.Session.AppID != mode || p.Session.ID != "fresh" {
					t.Errorf("route=%s session=%+v", route, p.Session)
				}
				if received >= len(frames) || string(p.RawMessage) != frames[received] {
					t.Errorf("event %d: %s", received, p.RawMessage)
				}
				received++
			}
			event.DefaultHandlers.Ready = func(p *dto.WSPayload, _ *dto.WSReadyData) { record("global", p) }
			event.DefaultHandlers.Plain = func(p *dto.WSPayload, _ []byte) error {
				record("global", p)
				return nil
			}
			session := dto.Session{AppID: mode, Shards: dto.ShardConfig{ShardCount: 1}}
			accept := func(ctx context.Context, p *dto.WSPayload) error {
				if err := ctx.Err(); err != nil {
					t.Error(err)
				}
				record(mode, p)
				return nil
			}
			switch mode {
			case "callback":
				session.EventHandler = accept
			case "dispatcher":
				session.EventHandler = event.NewDispatcher(accept).Handle
			}
			client := (&Client{}).New(session).(*Client)
			defer client.Close()
			for _, raw := range frames {
				var payload dto.WSPayload
				if err := json.Unmarshal([]byte(raw), &payload); err != nil {
					t.Fatal(err)
				}
				payload.RawMessage = []byte(raw)
				client.messageQueue <- &payload
			}
			close(client.messageQueue)
			client.listenMessageAndHandle()
			if received != len(frames) || client.Session().ID != "fresh" || client.Session().LastSeq != 3 {
				t.Fatalf("events=%d session=%+v", received, client.Session())
			}
		})
	}
}
