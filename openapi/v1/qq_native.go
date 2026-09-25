package v1

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/WindowsSov8forUs/botgo-plus/dto"
)

// Only guard nil values before invoking APIMessage methods. QQ validates message fields.
func validateNativeMessage(msg dto.APIMessage) error {
	if msg == nil {
		return errors.New("nil QQ message")
	}
	switch value := msg.(type) {
	case *dto.MessageToCreate:
		if value == nil {
			return errors.New("nil QQ message")
		}
	case *dto.RichMediaMessage:
		if value == nil {
			return errors.New("nil QQ media message")
		}
	}
	return nil
}

func nativeID(id string) (string, error) {
	if id == "" || id == "." || id == ".." {
		return "", errors.New("QQ identifier is required")
	}
	return url.PathEscape(id), nil
}
func nativeGroupPath(id, suffix string) (string, error) {
	value, err := nativeID(id)
	if err != nil {
		return "", err
	}
	return "/v2/groups/" + value + suffix, nil
}
func nativeUserPath(id, suffix string) (string, error) {
	value, err := nativeID(id)
	if err != nil {
		return "", err
	}
	return "/v2/users/" + value + suffix, nil
}

// GetQQGroupInfo and all QQGroup methods target QQ groups, never QQ guilds/channels.
// Platform permissions still apply even when an endpoint has a typed SDK method.
func (o *openAPI) GetQQGroupInfo(ctx context.Context, groupID string) (*dto.QQGroupInfo, *ResponseMeta, error) {
	path, err := nativeGroupPath(groupID, "/info")
	if err != nil {
		return nil, nil, err
	}
	var result dto.QQGroupInfo
	meta, err := o.Do(ctx, http.MethodGet, path, nil, &result)
	if err != nil {
		return nil, meta, err
	}
	if err := validateNativeResult(&result, meta.Raw); err != nil {
		return nil, meta, meta.WrapError("validate QQ API response", err)
	}
	return &result, meta, nil
}
func (o *openAPI) GetQQGroupMembers(ctx context.Context, groupID, cursor string) (*dto.QQGroupMembersPage, *ResponseMeta, error) {
	path, err := nativeGroupPath(groupID, "/members")
	if err != nil {
		return nil, nil, err
	}
	if cursor != "" {
		path += "?" + url.Values{"cursor": {cursor}}.Encode()
	}
	var result dto.QQGroupMembersPage
	meta, err := o.Do(ctx, http.MethodGet, path, nil, &result)
	if err != nil {
		return nil, meta, err
	}
	if err := validateNativeResult(&result, meta.Raw); err != nil {
		return nil, meta, meta.WrapError("validate QQ API response", err)
	}
	return &result, meta, nil
}
func (o *openAPI) GetQQGroupMember(ctx context.Context, groupID, memberID string) (*dto.QQGroupMember, *ResponseMeta, error) {
	id, err := nativeID(memberID)
	if err != nil {
		return nil, nil, err
	}
	path, err := nativeGroupPath(groupID, "/members/"+id)
	if err != nil {
		return nil, nil, err
	}
	var result dto.QQGroupMember
	meta, err := o.Do(ctx, http.MethodGet, path, nil, &result)
	if err != nil {
		return nil, meta, err
	}
	if err := validateNativeResult(&result, meta.Raw); err != nil {
		return nil, meta, meta.WrapError("validate QQ API response", err)
	}
	return &result, meta, nil
}
func (o *openAPI) RemoveQQGroupMembers(ctx context.Context, groupID string, request *dto.QQGroupRemoveRequest) (*dto.QQGroupRemoveResult, *ResponseMeta, error) {
	if request == nil || len(request.MemberOpenIDs) < 1 || len(request.MemberOpenIDs) > 20 {
		return nil, nil, errors.New("remove requires 1 to 20 members")
	}
	seen := make(map[string]bool)
	for _, id := range request.MemberOpenIDs {
		if id == "" || seen[id] {
			return nil, nil, errors.New("empty or duplicate member ID")
		}
		seen[id] = true
	}
	path, err := nativeGroupPath(groupID, "/batch_remove_members")
	if err != nil {
		return nil, nil, err
	}
	var result dto.QQGroupRemoveResult
	meta, err := o.Do(ctx, http.MethodPost, path, request, &result)
	if err != nil {
		return nil, meta, err
	}
	return &result, meta, nil
}
func (o *openAPI) GetQQGroupMuteState(ctx context.Context, groupID string) (*dto.QQGroupMuteState, *ResponseMeta, error) {
	path, err := nativeGroupPath(groupID, "/restrict_chat_setting")
	if err != nil {
		return nil, nil, err
	}
	var result dto.QQGroupMuteState
	meta, err := o.Do(ctx, http.MethodGet, path, nil, &result)
	if err != nil {
		return nil, meta, err
	}
	return &result, meta, nil
}

// SetQQGroupMemberMute performs only the supplied operations; it does not read and rewrite group state.
func (o *openAPI) SetQQGroupMemberMute(ctx context.Context, groupID string, request *dto.QQGroupMuteRequest) (*ResponseMeta, error) {
	if request == nil || len(request.Members) < 1 || len(request.Members) > 20 {
		return nil, errors.New("mute requires 1 to 20 member operations")
	}
	seen := make(map[string]bool)
	for _, item := range request.Members {
		if item.MemberOpenID == "" || seen[item.MemberOpenID] {
			return nil, errors.New("empty or duplicate member ID")
		}
		seen[item.MemberOpenID] = true
		if item.Op != "add" && item.Op != "update" && item.Op != "del" {
			return nil, errors.New("mute operation must be add, update or del")
		}
		if item.Op != "del" {
			if _, err := time.Parse(time.RFC3339, item.MuteExpireAt); err != nil {
				return nil, errors.New("mute_expire_at must be RFC3339")
			}
		}
	}
	path, err := nativeGroupPath(groupID, "/restrict_chat_setting")
	if err != nil {
		return nil, err
	}
	return o.Do(ctx, http.MethodPost, path, request, nil)
}

func (o *openAPI) PostC2CStreamMessage(ctx context.Context, userID string, request *dto.C2CStreamRequest) (*dto.Message, *ResponseMeta, error) {
	if request == nil {
		return nil, nil, errors.New("nil C2C stream request")
	}
	path, err := nativeUserPath(userID, "/stream_messages")
	if err != nil {
		return nil, nil, err
	}
	var result dto.Message
	meta, err := o.Do(ctx, http.MethodPost, path, request, &result)
	if err != nil {
		return nil, meta, err
	}
	return &result, meta, nil
}

// AcknowledgeInteraction is separate from the HTTP callback ACK. The caller chooses when to acknowledge.
func (o *openAPI) AcknowledgeInteraction(ctx context.Context, interactionID string, code int) (*ResponseMeta, error) {
	if code < 0 || code > 5 {
		return nil, errors.New("interaction result code must be between 0 and 5")
	}
	if strings.HasPrefix(interactionID, "INTERACTION_CREATE:") {
		return nil, errors.New("use the native interaction d.id without the event prefix")
	}
	id, err := nativeID(interactionID)
	if err != nil {
		return nil, err
	}
	return o.Do(ctx, http.MethodPut, "/interactions/"+id, struct {
		Code int `json:"code"`
	}{code}, nil)
}

func validateMediaRequest(r *dto.MediaUploadRequest) error {
	if r == nil {
		return errors.New("nil media upload")
	}
	if (r.URL == "") == (r.UploadID == "") {
		return errors.New("provide exactly one of url and upload_id")
	}
	if r.UploadID == "" && (r.FileType < 1 || r.FileType > 4) {
		return errors.New("file_type must be between 1 and 4")
	}
	if r.URL != "" {
		u, err := url.Parse(r.URL)
		if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
			return errors.New("media URL must be HTTP or HTTPS")
		}
	}
	return nil
}
func (o *openAPI) uploadNativeFile(ctx context.Context, path string, r *dto.MediaUploadRequest) (*dto.MediaUploadResult, *ResponseMeta, error) {
	if err := validateMediaRequest(r); err != nil {
		return nil, nil, err
	}
	var result dto.MediaUploadResult
	meta, err := o.Do(ctx, http.MethodPost, path, r, &result)
	if err != nil {
		return nil, meta, err
	}
	if err := validateNativeResult(&result, meta.Raw); err != nil {
		return nil, meta, meta.WrapError("validate QQ API response", err)
	}
	return &result, meta, nil
}
func (o *openAPI) UploadGroupFile(ctx context.Context, groupID string, r *dto.MediaUploadRequest) (*dto.MediaUploadResult, *ResponseMeta, error) {
	path, err := nativeGroupPath(groupID, "/files")
	if err != nil {
		return nil, nil, err
	}
	return o.uploadNativeFile(ctx, path, r)
}
func (o *openAPI) UploadC2CFile(ctx context.Context, userID string, r *dto.MediaUploadRequest) (*dto.MediaUploadResult, *ResponseMeta, error) {
	path, err := nativeUserPath(userID, "/files")
	if err != nil {
		return nil, nil, err
	}
	return o.uploadNativeFile(ctx, path, r)
}
func (o *openAPI) prepareNativeFile(ctx context.Context, path string, r *dto.UploadPrepareRequest) (*dto.UploadPrepareResult, *ResponseMeta, error) {
	if r == nil || r.FileType < 1 || r.FileType > 4 || r.FileSize <= 0 || r.FileName == "" || len(r.MD5) != 32 || len(r.SHA1) != 40 || len(r.MD5Prefix) != 32 {
		return nil, nil, errors.New("invalid upload preparation parameters")
	}
	var result dto.UploadPrepareResult
	meta, err := o.Do(ctx, http.MethodPost, path, r, &result)
	if err != nil {
		return nil, meta, err
	}
	return &result, meta, nil
}
func (o *openAPI) PrepareGroupUpload(ctx context.Context, groupID string, r *dto.UploadPrepareRequest) (*dto.UploadPrepareResult, *ResponseMeta, error) {
	path, err := nativeGroupPath(groupID, "/upload_prepare")
	if err != nil {
		return nil, nil, err
	}
	return o.prepareNativeFile(ctx, path, r)
}
func (o *openAPI) PrepareC2CUpload(ctx context.Context, userID string, r *dto.UploadPrepareRequest) (*dto.UploadPrepareResult, *ResponseMeta, error) {
	path, err := nativeUserPath(userID, "/upload_prepare")
	if err != nil {
		return nil, nil, err
	}
	return o.prepareNativeFile(ctx, path, r)
}
func (o *openAPI) finishNativePart(ctx context.Context, path string, r *dto.UploadPartFinishRequest) (*ResponseMeta, error) {
	if r == nil || r.UploadID == "" || r.PartIndex < 0 || r.BlockSize <= 0 || len(r.MD5) != 32 {
		return nil, errors.New("invalid upload part confirmation")
	}
	return o.Do(ctx, http.MethodPost, path, r, nil)
}
func (o *openAPI) FinishGroupUploadPart(ctx context.Context, groupID string, r *dto.UploadPartFinishRequest) (*ResponseMeta, error) {
	path, err := nativeGroupPath(groupID, "/upload_part_finish")
	if err != nil {
		return nil, err
	}
	return o.finishNativePart(ctx, path, r)
}
func (o *openAPI) FinishC2CUploadPart(ctx context.Context, userID string, r *dto.UploadPartFinishRequest) (*ResponseMeta, error) {
	path, err := nativeUserPath(userID, "/upload_part_finish")
	if err != nil {
		return nil, err
	}
	return o.finishNativePart(ctx, path, r)
}

// Check only the fields needed to identify a successful result; unknown fields are allowed.
func validateNativeResult(result any, raw []byte) error {
	switch value := result.(type) {
	case *dto.QQGroupInfo:
		if value.GroupOpenID == "" {
			return errors.New("QQ group response has no group_openid")
		}
	case *dto.QQGroupMember:
		if value.MemberOpenID == "" {
			return errors.New("QQ member response has no member_openid")
		}
	case *dto.QQGroupMembersPage:
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(raw, &fields); err != nil {
			return err
		}
		members := bytes.TrimSpace(fields["members"])
		if len(members) == 0 || bytes.Equal(members, []byte("null")) {
			return errors.New("QQ member page has no members array")
		}
		for _, member := range value.Members {
			if member.MemberOpenID == "" {
				return errors.New("QQ member page contains an entry without member_openid")
			}
		}
	case *dto.MediaUploadResult:
		if value.FileInfo == "" {
			return errors.New("QQ file response has no file_info")
		}
	}
	return nil
}
