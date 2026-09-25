package v1

import (
	"context"

	"github.com/WindowsSov8forUs/botgo-plus/dto"
	"github.com/WindowsSov8forUs/botgo-plus/log"
)

// PostAudio AudioAPI 接口实现
func (o *openAPI) PostAudio(ctx context.Context, channelID string, value *dto.AudioControl) (*dto.AudioControl, error) {
	// 目前服务端成功不回包
	resp, err := o.request(ctx).
		SetResult(dto.Channel{}).
		SetPathParam("channel_id", channelID).
		SetBody(value).
		Post(o.getURL(audioControlURI))
	err = responseError(resp, err)
	if err != nil {
		return nil, err
	}

	return value, nil
}

// PutMic 上麦接口实现
func (o *openAPI) PutMic(ctx context.Context, channelID string) error {
	resp, err := o.request(ctx).
		SetPathParam("channel_id", channelID).
		Put(o.getURL(micURI))
	err = responseError(resp, err)
	if err != nil {
		log.Errorf("put mic fail:%+v", err)
	}
	return err
}

// DeleteMic 上麦接口实现
func (o *openAPI) DeleteMic(ctx context.Context, channelID string) error {
	resp, err := o.request(ctx).
		SetPathParam("channel_id", channelID).
		Delete(o.getURL(micURI))
	err = responseError(resp, err)
	if err != nil {
		log.Errorf("delete mic fail:%+v", err)
	}
	return err
}
