package dto

import (
	"encoding/json"
	"strconv"
)

// DecimalInt64 is serialized as a decimal string, as required by QQ upload APIs.
// It also accepts numeric responses from compatible gateway versions.
type DecimalInt64 int64

func (v DecimalInt64) MarshalJSON() ([]byte, error) {
	return json.Marshal(strconv.FormatInt(int64(v), 10))
}
func (v *DecimalInt64) UnmarshalJSON(data []byte) error {
	s := string(data)
	if len(data) > 0 && data[0] == '"' {
		if err := json.Unmarshal(data, &s); err != nil {
			return err
		}
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return err
	}
	*v = DecimalInt64(n)
	return nil
}

type QQGroupInfo struct {
	GroupOpenID     string   `json:"group_openid"`
	GroupName       string   `json:"group_name"`
	GroupFingerMemo string   `json:"group_finger_memo,omitempty"`
	GroupClassText  string   `json:"group_class_text,omitempty"`
	GroupTags       []string `json:"group_tags,omitempty"`
	GroupMemberNum  int      `json:"group_member_num"`
}

type QQGroupMember struct {
	MemberOpenID string `json:"member_openid"`
	Username     string `json:"username"`
	MemberRole   string `json:"member_role"`
	Bot          bool   `json:"bot"`
	JoinedAt     string `json:"joined_at"`
	UnionOpenID  string `json:"union_openid,omitempty"`
}

type QQGroupMembersPage struct {
	Members    []QQGroupMember `json:"members"`
	NextCursor string          `json:"next_cursor"`
}

type QQGroupRemoveRequest struct {
	MemberOpenIDs        []string `json:"member_openids"`
	AddToMemberBlacklist bool     `json:"add_to_member_blacklist,omitempty"`
}

type QQGroupRemoveResult struct {
	RemoveMembersResult    string   `json:"remove_members_result"`
	BlacklistFailedOpenIDs []string `json:"add_to_member_blacklist_fail_openids"`
}

type QQMutedMember struct {
	MemberOpenID string `json:"member_openid"`
	MuteExpireAt string `json:"mute_expire_at"`
	Username     string `json:"username,omitempty"`
	UnionOpenID  string `json:"union_openid,omitempty"`
}

type QQGroupMuteState struct {
	GlobalRule json.RawMessage `json:"global_rule"`
	Members    []QQMutedMember `json:"members"`
}

type QQGroupMuteOperation struct {
	Op           string `json:"op"`
	MemberOpenID string `json:"member_openid"`
	MuteExpireAt string `json:"mute_expire_at,omitempty"`
}

type QQGroupMuteRequest struct {
	Members []QQGroupMuteOperation `json:"members"`
}

// C2CStreamRequest is not interchangeable with the historical Stream field.
// Index zero must be transmitted for the first chunk. Retry identical chunks only.
type C2CStreamRequest struct {
	InputMode   string `json:"input_mode,omitempty"`
	InputState  int    `json:"input_state"`
	Index       uint64 `json:"index"`
	ContentType string `json:"content_type,omitempty"`
	ContentRaw  string `json:"content_raw"`
	EventID     string `json:"event_id,omitempty"`
	MsgID       string `json:"msg_id,omitempty"`
	StreamMsgID string `json:"stream_msg_id,omitempty"`
	MsgSeq      uint32 `json:"msg_seq,omitempty"`
	IsWakeup    bool   `json:"is_wakeup,omitempty"`
}

// MediaUploadRequest chooses either a URL or a completed upload ID.
// SrvSendMsg defaults to false; explicitly setting it true requests automatic sending.
type MediaUploadRequest struct {
	FileType   int    `json:"file_type,omitempty"`
	URL        string `json:"url,omitempty"`
	FileName   string `json:"file_name,omitempty"`
	UploadID   string `json:"upload_id,omitempty"`
	SrvSendMsg bool   `json:"srv_send_msg"`
}

type MediaUploadResult struct {
	FileUUID string `json:"file_uuid"`
	FileInfo string `json:"file_info"`
	TTL      uint64 `json:"ttl"`
	ID       string `json:"id,omitempty"`
	RawURL   string `json:"raw_url,omitempty"`
}

type UploadPrepareRequest struct {
	FileType  int          `json:"file_type"`
	FileSize  DecimalInt64 `json:"file_size"`
	FileName  string       `json:"file_name"`
	MD5       string       `json:"md5"`
	SHA1      string       `json:"sha1"`
	MD5Prefix string       `json:"md5_10m"`
}

type UploadPart struct {
	Index        int          `json:"index"`
	PresignedURL string       `json:"presigned_url"`
	BlockSize    DecimalInt64 `json:"block_size,omitempty"`
}

type UploadConfig struct {
	Concurrency  int `json:"concurrency"`
	RetryTimeout int `json:"retry_timeout"`
	RetryDelay   int `json:"retry_delay"`
}

type UploadPrepareResult struct {
	UploadID     string       `json:"upload_id"`
	BlockSize    DecimalInt64 `json:"block_size"`
	Parts        []UploadPart `json:"parts"`
	UploadConfig UploadConfig `json:"upload_config"`
}

type UploadPartFinishRequest struct {
	UploadID  string       `json:"upload_id"`
	PartIndex int          `json:"part_index"`
	BlockSize DecimalInt64 `json:"block_size"`
	MD5       string       `json:"md5"`
}
