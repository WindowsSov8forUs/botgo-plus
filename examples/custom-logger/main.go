package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	botgo "github.com/WindowsSov8forUs/botgo-plus"
	"github.com/WindowsSov8forUs/botgo-plus/dto"
	"github.com/WindowsSov8forUs/botgo-plus/dto/message"
	"github.com/WindowsSov8forUs/botgo-plus/event"
	iwebhook "github.com/WindowsSov8forUs/botgo-plus/interaction/webhook"
	"github.com/WindowsSov8forUs/botgo-plus/openapi"
	"github.com/WindowsSov8forUs/botgo-plus/token"
)

const (
	host = "0.0.0.0"
	port = 9000
	path = "/qqbot"
)

type config struct {
	AppID  uint64 `yaml:"appid"`
	Secret string `yaml:"secret"`
	Token  string `yaml:"token"`
}

func main() {
	logger, err := New("./", DebugLevel)
	if err != nil {
		log.Fatalln("error log new", err)
	}
	botgo.SetLogger(logger)

	content, err := os.ReadFile("config.yaml")
	if err != nil {
		log.Fatalln("load config file failed, err:", err)
	}

	conf := &config{}
	if err = yaml.Unmarshal(content, conf); err != nil {
		log.Fatalln("parse config failed, err:", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tk := token.BotToken(conf.AppID, conf.Secret, conf.Token, token.TypeQQBot)
	if err = tk.InitToken(ctx); err != nil {
		log.Fatalln(err)
	}

	api := botgo.NewOpenAPI(tk).WithTimeout(5 * time.Second)
	_ = event.RegisterHandlers(GuildATMessageEventHandler(api))

	iwebhook.DefaultGetSecretFunc = func() string { return conf.Secret }
	http.HandleFunc(path, iwebhook.HTTPHandler)

	if err = http.ListenAndServe(fmt.Sprintf("%s:%d", host, port), nil); err != nil {
		log.Fatal("setup server fatal:", err)
	}
}

// GuildATMessageEventHandler handles AT messages.
func GuildATMessageEventHandler(api openapi.OpenAPI) event.ATMessageEventHandler {
	_ = api
	return func(event *dto.Payload, data *dto.ATMessageData) error {
		log.Printf("[%s] %s", event.Type, data.Content)
		input := strings.ToLower(message.ETLInput(data.Content))
		log.Printf("clear input content is: %s", input)
		return nil
	}
}
