package dto

import (
	"encoding/json"
	"strings"
)

// ARKData retains the native card payload without rendering it into another protocol.
type ARKData struct {
	Prompt  string                     `json:"prompt,omitempty"`
	ArkType string                     `json:"ark_type,omitempty"`
	ArkName string                     `json:"ark_name,omitempty"`
	Fields  map[string]json.RawMessage `json:"fields,omitempty"`
}

type MessageElement struct {
	MsgIdx      string               `json:"msg_idx,omitempty"`
	Author      *User                `json:"author,omitempty"`
	MessageType int                  `json:"message_type,omitempty"`
	Content     string               `json:"content,omitempty"`
	Attachments []*MessageAttachment `json:"attachments,omitempty"`
	ArkData     *ARKData             `json:"ark_data,omitempty"`
	MsgElements []MessageElement     `json:"msg_elements,omitempty"`
}

type MessageExtInfo struct {
	RefIdx string `json:"ref_idx,omitempty"`
}

// UnmarshalJSON keeps the original body, including fields unknown to this SDK version.
// Raw may include auth_token or personal data and must not be logged by default.
func (m *Message) UnmarshalJSON(data []byte) error {
	type wire Message
	var value wire
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	*m = Message(value)
	m.Raw = append([]byte(nil), data...)
	return nil
}

// GetExt splits once so values ending with one or more '=' characters remain intact.
func (s MessageScene) GetExt(key string) (string, bool) {
	for _, entry := range s.Ext {
		k, value, ok := strings.Cut(entry, "=")
		if ok && k == key {
			return value, true
		}
	}
	return "", false
}

func (m *Message) EffectiveGroupID() string {
	if m == nil {
		return ""
	}
	if m.GroupOpenID != "" {
		return m.GroupOpenID
	}
	return m.GroupID
}

// EffectiveUserID provides a local fallback, not a cross-app identity conversion.
func (u *User) EffectiveUserID() string {
	if u == nil {
		return ""
	}
	if u.UserOpenID != "" {
		return u.UserOpenID
	}
	if u.MemberOpenID != "" {
		return u.MemberOpenID
	}
	return u.ID
}

func (m *WSMessageData) UnmarshalJSON(data []byte) error   { return (*Message)(m).UnmarshalJSON(data) }
func (m *WSATMessageData) UnmarshalJSON(data []byte) error { return (*Message)(m).UnmarshalJSON(data) }
func (m *WSDirectMessageData) UnmarshalJSON(data []byte) error {
	return (*Message)(m).UnmarshalJSON(data)
}
func (m *WSGroupATMessageData) UnmarshalJSON(data []byte) error {
	return (*Message)(m).UnmarshalJSON(data)
}
func (m *WSC2CMessageData) UnmarshalJSON(data []byte) error { return (*Message)(m).UnmarshalJSON(data) }
