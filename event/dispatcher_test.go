package event

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/WindowsSov8forUs/botgo-plus/dto"
)

func TestEventDispatch(t *testing.T) {
	raw := []byte(`{"op":0,"t":"GROUP_MESSAGE_CREATE","d":{"id":"message","group_openid":"group"}}`)
	var payload dto.WSPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatal(err)
	}
	payload.RawMessage = raw
	var received string
	dispatcher := NewDispatcher(func(_ context.Context, p *dto.WSPayload) error {
		received = string(p.Type)
		return nil
	})
	RegisterTyped(dispatcher, dto.EventGroupMessageCreate, func(_ context.Context, _ *dto.WSPayload, message *dto.WSGroupMessageData) error {
		received = message.GroupOpenID + "/" + message.ID
		return nil
	})
	if err := dispatcher.Handle(context.Background(), &payload); err != nil {
		t.Fatal(err)
	}
	if received != "group/message" {
		t.Fatalf("typed event=%s", received)
	}
	payload.Type = "FUTURE_EVENT"
	if err := dispatcher.Handle(context.Background(), &payload); err != nil {
		t.Fatal(err)
	}
	if received != "FUTURE_EVENT" {
		t.Fatalf("raw event=%s", received)
	}
	previous := DefaultHandlers
	defer func() { DefaultHandlers = previous }()
	intent := RegisterHandlers(GroupMessageEventHandler(func(_ *dto.WSPayload, message *dto.WSGroupMessageData) error {
		received = message.ID
		return nil
	}))
	payload.Type = dto.EventGroupMessageCreate
	if err := ParseAndHandle(&payload); err != nil {
		t.Fatal(err)
	}
	if intent != dto.IntentGroupMessages || received != "message" {
		t.Fatalf("registered event=%s intent=%v", received, intent)
	}
}
