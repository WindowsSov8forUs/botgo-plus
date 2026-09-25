package v1

import (
	"context"
)

// HeaderCallbackAppID 互动按钮第三方回调 appID
const HeaderCallbackAppID = "X-Callback-AppID"

// PutInteraction 更新 interaction
func (o *openAPI) PutInteraction(ctx context.Context,
	interactionID string, body string) error {
	resp, err := o.request(ctx).
		SetHeader(HeaderCallbackAppID, o.GetAppID()).
		SetPathParam("interaction_id", interactionID).
		SetBody(body).
		Put(o.getURL(interactionsURI))
	err = responseError(resp, err)
	return err
}
