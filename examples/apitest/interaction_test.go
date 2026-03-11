package apitest

import (
	"encoding/json"
	"testing"

	"github.com/WindowsSov8forUs/botgo-plus/dto"
)

func TestInteractions(t *testing.T) {
	t.Run(
		"put interaction", func(t *testing.T) {
			body, _ := json.Marshal(
				dto.InteractionData{
					Name: "interaction",
					Type: 2,
					Resolved: struct {
						ButtonData string `json:"button_data,omitempty"`
						ButtonID   string `json:"button_id,omitempty"`
						UserID     string `json:"user_id,omitempty"`
						FeatureID  string `json:"feature_id,omitempty"`
						MessageID  string `json:"message_id,omitempty"`
					}{
						ButtonData: "test",
					},
				},
			)
			err := api.PutInteraction(ctx, testInteractionD, string(body))
			if err != nil {
				t.Error(err)
			}
		},
	)
}
