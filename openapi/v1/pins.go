package v1

import (
	"context"

	"github.com/WindowsSov8forUs/botgo-plus/dto"
)

// AddPins 添加精华消息
func (o *openAPI) AddPins(ctx context.Context, channelID string, messageID string) (*dto.PinsMessage, error) {
	resp, err := o.request(ctx).
		SetResult(dto.PinsMessage{}).
		SetPathParam("channel_id", channelID).
		SetPathParam("message_id", messageID).
		Put(o.getURL(pinURI))
	err = responseError(resp, err)
	if err != nil {
		return nil, err
	}
	return resp.Result().(*dto.PinsMessage), nil
}

// DeletePins 删除精华消息
func (o *openAPI) DeletePins(ctx context.Context, channelID, messageID string) error {
	resp, err := o.request(ctx).
		SetResult(dto.PinsMessage{}).
		SetPathParam("channel_id", channelID).
		SetPathParam("message_id", messageID).
		Delete(o.getURL(pinURI))
	err = responseError(resp, err)
	return err
}

// GetPins 获取精华消息
func (o *openAPI) GetPins(ctx context.Context, channelID string) (*dto.PinsMessage, error) {
	resp, err := o.request(ctx).
		SetResult(dto.PinsMessage{}).
		SetPathParam("channel_id", channelID).
		Get(o.getURL(pinsURI))
	err = responseError(resp, err)
	if err != nil {
		return nil, err
	}
	return resp.Result().(*dto.PinsMessage), nil
}

// CleanPins 清除全部精华消息
func (o *openAPI) CleanPins(ctx context.Context, channelID string) error {
	resp, err := o.request(ctx).
		SetResult(dto.PinsMessage{}).
		SetPathParam("channel_id", channelID).
		SetPathParam("message_id", "all").
		Delete(o.getURL(pinURI))
	err = responseError(resp, err)
	return err
}
