package apitest

import (
	"context"
	"log"
	"os"
	"testing"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/WindowsSov8forUs/botgo-plus"
	"github.com/WindowsSov8forUs/botgo-plus/openapi"
	"github.com/WindowsSov8forUs/botgo-plus/token"
)

var conf struct {
	AppID  uint64 `yaml:"appid"`
	Secret string `yaml:"secret"`
	Token  string `yaml:"token"`
}
var api openapi.OpenAPI

var (
	testGuildID   = "3326534247441079828" // replace your guild id
	testChannelID = "1595028"             // replace your channel id
	testMessageID = `08e092eeb983afef9e0110f9bb5d1a1231343431313532313836373838333234303420801e
28003091c4bb02380c400c48d8a7928d06` // replace your channel id
	testRolesID            = `10054557`                             // replace your roles id
	testMemberID           = `1201318637970874066`                  // replace your member id
	testMarkdownTemplateID = 1231231231231231                       // replace your markdown template id
	testInteractionD       = "e924431f-aaed-4e78-8375-9295b1f1e0ef" // replace your interaction id
	ctx                    context.Context
)

func TestMain(m *testing.M) {
	ctx = context.Background()
	content, err := os.ReadFile("./config.yaml")
	if err != nil {
		log.Println("read conf failed, skip apitest:", err)
		os.Exit(0)
	}
	if err := yaml.Unmarshal(content, &conf); err != nil {
		log.Println("parse conf failed, skip apitest:", err)
		os.Exit(0)
	}
	tk := token.BotToken(conf.AppID, conf.Secret, conf.Token, token.TypeQQBot)
	if err := tk.InitToken(ctx); err != nil {
		log.Println("init token failed, skip apitest:", err)
		os.Exit(0)
	}
	api = botgo.NewOpenAPI(tk).WithTimeout(3 * time.Second)
	os.Exit(m.Run())
}
