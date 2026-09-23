package dto

import (
	"encoding/json"
	"testing"
)

func TestNativeMessage(t *testing.T) {
	raw := []byte(`{"id":"message","group_openid":"group","author":{"member_openid":"member","member_role":"admin"},"message_scene":{"ext":["msg_idx=REFIDX_a=="]},"message_type":102,"msg_elements":[{"message_type":101,"msg_elements":[{"message_type":3,"ark_data":{"ark_name":"card","fields":{"count":4}}}]}],"attachments":[{"content_type":"voice","voice_wav_url":"https://example.invalid/audio","asr_refer_text":"text"}]}`)
	var message WSGroupMessageData
	if err := json.Unmarshal(raw, &message); err != nil {
		t.Fatal(err)
	}
	if message.ID != "message" || message.GroupOpenID != "group" || message.Author == nil || message.Author.MemberOpenID != "member" || message.Author.MemberRole != "admin" {
		t.Fatalf("message identity: %+v", message)
	}
	if value, ok := message.MessageScene.GetExt("msg_idx"); !ok || value != "REFIDX_a==" {
		t.Fatalf("message index: %q", value)
	}
	if len(message.MsgElements) != 1 || len(message.MsgElements[0].MsgElements) != 1 {
		t.Fatalf("message elements: %+v", message.MsgElements)
	}
	card := message.MsgElements[0].MsgElements[0].ArkData
	if card == nil || card.ArkName != "card" || string(card.Fields["count"]) != "4" {
		t.Fatalf("card: %+v", card)
	}
	if len(message.Attachments) != 1 || message.Attachments[0].ASRReferText != "text" || string(message.Raw) != string(raw) {
		t.Fatalf("attachments/raw: %+v", message.Attachments)
	}
	for kind, want := range map[EventType]Intent{
		EventGroupMessageCreate: IntentGroupMessages,
		EventGroupMemberAdd:     IntentGroupMembers,
		EventGroupJoinRequest:   IntentGroupMembers,
		EventChannelCreate:      IntentGuilds,
		EventAudioFinish:        IntentAudio,
	} {
		if got := EventToIntent(kind); got != want {
			t.Errorf("%s intent=%v, want %v", kind, got, want)
		}
	}
}

func TestWireEncoding(t *testing.T) {
	for _, tc := range []struct {
		name  string
		value interface{}
		want  string
	}{
		{"media", MediaInfo{FileInfo: "opaque!file-info"}, `{"file_info":"opaque!file-info"}`},
		{"stream", C2CStreamRequest{InputState: 1, ContentRaw: "text"}, `{"input_state":1,"index":0,"content_raw":"text"}`},
		{"upload-size", DecimalInt64(10002432), `"10002432"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := json.Marshal(tc.value)
			if err != nil || string(got) != tc.want {
				t.Fatalf("encoded=%s, want %s: %v", got, tc.want, err)
			}
		})
	}
	for _, raw := range []string{`10002432`, `"10002432"`} {
		var value DecimalInt64
		if err := json.Unmarshal([]byte(raw), &value); err != nil || value != 10002432 {
			t.Fatalf("decoded=%d: %v", value, err)
		}
	}
}
