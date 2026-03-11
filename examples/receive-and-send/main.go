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
	"github.com/WindowsSov8forUs/botgo-plus/token"
)

const (
	host = "0.0.0.0"
	port = 9000
	path = "/qqbot"
)

// Processor used by handlers.
var processor Processor

type config struct {
	AppID  uint64 `yaml:"appid"`
	Secret string `yaml:"secret"`
	Token  string `yaml:"token"`
}

func main() {
	content, err := os.ReadFile("config.yaml")
	if err != nil {
		log.Fatalln("load config file failed, err:", err)
	}

	conf := &config{}
	if err = yaml.Unmarshal(content, conf); err != nil {
		log.Fatalln("parse config failed, err:", err)
	}
	log.Println("config loaded")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tk := token.BotToken(conf.AppID, conf.Secret, conf.Token, token.TypeQQBot)
	if err = tk.InitToken(ctx); err != nil {
		log.Fatalln(err)
	}

	api := botgo.NewOpenAPI(tk).WithTimeout(5 * time.Second)
	processor = Processor{api: api}

	_ = event.RegisterHandlers(
		GroupATMessageEventHandler(),
		C2CMessageEventHandler(),
		ChannelATMessageEventHandler(),
	)

	iwebhook.DefaultGetSecretFunc = func() string { return conf.Secret }
	http.HandleFunc(path, iwebhook.HTTPHandler)

	if err = http.ListenAndServe(fmt.Sprintf("%s:%d", host, port), nil); err != nil {
		log.Fatal("setup server fatal:", err)
	}
}

// ChannelATMessageEventHandler handles channel at messages.
func ChannelATMessageEventHandler() event.ATMessageEventHandler {
	return func(event *dto.Payload, data *dto.ATMessageData) error {
		input := strings.ToLower(message.ETLInput(data.Content))
		return processor.ProcessChannelMessage(input, data)
	}
}

// InteractionHandler handles interaction events.
func InteractionHandler() event.InteractionEventHandler {
	return func(event *dto.Payload, data *dto.InteractionEventData) error {
		fmt.Println(data)
		return processor.ProcessInlineSearch(data)
	}
}

// GroupATMessageEventHandler handles group at messages.
func GroupATMessageEventHandler() event.GroupATMessageEventHandler {
	return func(event *dto.Payload, data *dto.GroupATMessageData) error {
		input := strings.ToLower(message.ETLInput(data.Content))
		return processor.ProcessGroupMessage(input, data)
	}
}

// C2CMessageEventHandler handles c2c messages.
func C2CMessageEventHandler() event.C2CMessageEventHandler {
	return func(event *dto.Payload, data *dto.C2CMessageData) error {
		return processor.ProcessC2CMessage(string(event.RawMessage), data)
	}
}

func GuildEventHandler() event.GuildEventHandler {
	return func(event *dto.Payload, data *dto.GuildData) error {
		fmt.Println(data)
		return nil
	}
}

func ChannelEventHandler() event.ChannelEventHandler {
	return func(event *dto.Payload, data *dto.ChannelData) error {
		fmt.Println(data)
		return nil
	}
}

func GuildMemberEventHandler() event.GuildMemberEventHandler {
	return func(event *dto.Payload, data *dto.GuildMemberData) error {
		fmt.Println(data)
		return nil
	}
}

func GuildDirectMessageHandler() event.DirectMessageEventHandler {
	return func(event *dto.Payload, data *dto.DirectMessageData) error {
		fmt.Println(data)
		return nil
	}
}

func GuildMessageHandler() event.MessageEventHandler {
	return func(event *dto.Payload, data *dto.MessageData) error {
		fmt.Println(data)
		return nil
	}
}
