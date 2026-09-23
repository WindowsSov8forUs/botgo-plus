package botgo

import (
	"testing"

	"github.com/WindowsSov8forUs/botgo-plus/openapi"
	"github.com/WindowsSov8forUs/botgo-plus/token"
)

func TestNewClient(t *testing.T) {
	source := token.NewQQBotTokenSource(&token.QQBotCredentials{AppID: "fixture", AppSecret: "fixture"})
	client, err := NewClient("fixture", source)
	if err != nil {
		t.Fatal(err)
	}
	if client.GetAppID() != "fixture" || client.Version() != openapi.APIv1 {
		t.Fatalf("client app=%s version=%v", client.GetAppID(), client.Version())
	}
}
