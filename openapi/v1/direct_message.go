package v1

import (
	"context"
	"net/http"

	"github.com/WindowsSov8forUs/botgo-plus/dto"
	"github.com/WindowsSov8forUs/botgo-plus/openapi/options"
)

// CreateDirectMessage 创建私信频道
func (o *openAPI) CreateDirectMessage(ctx context.Context,
	dm *dto.DirectMessageToCreate, opt ...options.Option) (*dto.DirectMessage, error) {
	reqCMD := o.request(ctx).
		SetResult(dto.DirectMessage{}).
		SetBody(dm)

	resp, err := baseRequest(ctx, reqCMD, http.MethodPost, o.getURL(userMeDMURI), opt...)
	err = responseError(resp, err)
	if err != nil {
		return nil, err
	}
	return resp.Result().(*dto.DirectMessage), nil
}

// PostDirectMessage 在私信频道内发消息
func (o *openAPI) PostDirectMessage(ctx context.Context,
	dm *dto.DirectMessage, msg *dto.MessageToCreate, opt ...options.Option) (*dto.Message, error) {
	reqCMD := o.request(ctx).
		SetResult(dto.Message{}).
		SetPathParam("guild_id", dm.GuildID).
		SetBody(msg)

	resp, err := baseRequest(ctx, reqCMD, http.MethodPost, o.getURL(dmsURI), opt...)
	err = responseError(resp, err)
	if err != nil {
		return nil, err
	}
	return resp.Result().(*dto.Message), nil
}

// RetractDMMessage 撤回私信消息
func (o *openAPI) RetractDMMessage(ctx context.Context,
	guildID, msgID string, opt ...options.Option) error {
	reqCMD := o.request(ctx).
		SetPathParam("guild_id", guildID).
		SetPathParam("message_id", msgID)

	resp, err := baseRequest(ctx, reqCMD, http.MethodDelete, o.getURL(dmsMessageURI), opt...)
	err = responseError(resp, err)
	return err
}

// PostDMSettingGuide 发送私信设置引导, jumpGuildID为设置引导要跳转的频道ID
func (o *openAPI) PostDMSettingGuide(ctx context.Context,
	dm *dto.DirectMessage, jumpGuildID string, opt ...options.Option) (*dto.Message, error) {
	msg := &dto.SettingGuideToCreate{
		SettingGuide: &dto.SettingGuide{
			GuildID: jumpGuildID,
		},
	}
	reqCMD := o.request(ctx).
		SetResult(dto.Message{}).
		SetPathParam("guild_id", dm.GuildID).
		SetBody(msg)

	resp, err := baseRequest(ctx, reqCMD, http.MethodPost, o.getURL(dmSettingGuideURI), opt...)
	err = responseError(resp, err)
	if err != nil {
		return nil, err
	}
	return resp.Result().(*dto.Message), nil
}
