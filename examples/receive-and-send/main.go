package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/WindowsSov8forUs/botgo-plus"
	"github.com/WindowsSov8forUs/botgo-plus/dto"
	"github.com/WindowsSov8forUs/botgo-plus/dto/message"
	"github.com/WindowsSov8forUs/botgo-plus/event"
	"github.com/WindowsSov8forUs/botgo-plus/interaction/webhook"
	sdklog "github.com/WindowsSov8forUs/botgo-plus/log"
	"github.com/WindowsSov8forUs/botgo-plus/token"
	"gopkg.in/yaml.v3"
)

const (
	host_ = "0.0.0.0"
	port_ = 9000
	path_ = "/qqbot"
)

// 消息处理器，持有 openapi 对象
var processor Processor

func main() {
	// 加载 appid 和 token
	content, err := os.ReadFile("config.yaml")
	if err != nil {
		log.Fatalln("load config file failed, err:", err)
	}
	credentials := &token.QQBotCredentials{
		AppID:     "",
		AppSecret: "",
	}
	if err = yaml.Unmarshal(content, &credentials); err != nil {
		log.Fatalln("parse config failed, err:", err)
	}
	log.Printf("loaded credentials for app %s", sdklog.SafeText(credentials.AppID))
	tokenSource := token.NewQQBotTokenSource(credentials)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel() //释放刷新协程
	if err = token.StartRefreshAccessToken(ctx, tokenSource); err != nil {
		log.Fatalf("start access token refresh failed: %s", sdklog.SafeError(err))
	}
	// 初始化 openapi，正式环境
	api := botgo.NewOpenAPI(credentials.AppID, tokenSource).WithTimeout(5 * time.Second).SetDebug(true)
	processor = Processor{api: api}
	// 注册处理函数
	_ = event.RegisterHandlers(
		// ***********消息事件***********
		// 群@机器人消息事件
		GroupATMessageEventHandler(),
		// C2C消息事件
		C2CMessageEventHandler(),
		// 频道@机器人事件
		ChannelATMessageEventHandler(),
	)
	http.HandleFunc(path_, func(writer http.ResponseWriter, request *http.Request) {
		webhook.HTTPHandler(writer, request, credentials)
	})
	if err = http.ListenAndServe(fmt.Sprintf("%s:%d", host_, port_), nil); err != nil {
		log.Fatalf("HTTP server stopped: %s", sdklog.SafeError(err))
	}
}

// ChannelATMessageEventHandler 实现处理 at 消息的回调
func ChannelATMessageEventHandler() event.ATMessageEventHandler {
	return func(event *dto.WSPayload, data *dto.WSATMessageData) error {
		input := strings.ToLower(message.ETLInput(data.Content))
		return processor.ProcessChannelMessage(input, data)
	}
}

// InteractionHandler 处理内联交互事件
func InteractionHandler() event.InteractionEventHandler {
	return func(event *dto.WSPayload, data *dto.WSInteractionData) error {
		if data != nil {
			fmt.Printf("received interaction %s\n", sdklog.SafeText(data.ID))
		}
		return processor.ProcessInlineSearch(data)
	}
}

// GroupATMessageEventHandler 实现处理 at 消息的回调
func GroupATMessageEventHandler() event.GroupATMessageEventHandler {
	return func(event *dto.WSPayload, data *dto.WSGroupATMessageData) error {
		input := strings.ToLower(message.ETLInput(data.Content))
		return processor.ProcessGroupMessage(input, data)
	}
}

// C2CMessageEventHandler 实现处理 at 消息的回调
func C2CMessageEventHandler() event.C2CMessageEventHandler {
	return func(event *dto.WSPayload, data *dto.WSC2CMessageData) error {
		return processor.ProcessC2CMessage(string(event.RawMessage), data)
	}
}

// C2CFriendEventHandler 实现处理好友关系变更的回调
func C2CFriendEventHandler() event.C2CFriendEventHandler {
	return func(event *dto.WSPayload, data *dto.WSC2CFriendData) error {
		if event != nil && data != nil {
			fmt.Printf("received friend event %s for user %s\n", sdklog.SafeText(string(event.Type)), sdklog.SafeText(data.OpenID))
		}
		return processor.ProcessFriend(string(event.Type), data)
	}
}

// GuildEventHandler 处理频道事件
func GuildEventHandler() event.GuildEventHandler {
	return func(event *dto.WSPayload, data *dto.WSGuildData) error {
		if event != nil && data != nil {
			fmt.Printf("received guild event %s for guild %s\n", sdklog.SafeText(string(event.Type)), sdklog.SafeText(data.ID))
		}
		return nil
	}
}

// ChannelEventHandler 处理子频道事件
func ChannelEventHandler() event.ChannelEventHandler {
	return func(event *dto.WSPayload, data *dto.WSChannelData) error {
		if event != nil && data != nil {
			fmt.Printf("received channel event %s for channel %s\n", sdklog.SafeText(string(event.Type)), sdklog.SafeText(data.ID))
		}
		return nil
	}
}

// GuildMemberEventHandler 处理成员变更事件
func GuildMemberEventHandler() event.GuildMemberEventHandler {
	return func(event *dto.WSPayload, data *dto.WSGuildMemberData) error {
		if event != nil && data != nil {
			fmt.Printf("received guild member event %s in guild %s\n", sdklog.SafeText(string(event.Type)), sdklog.SafeText(data.GuildID))
		}
		return nil
	}
}

// GuildDirectMessageHandler 处理频道私信事件
func GuildDirectMessageHandler() event.DirectMessageEventHandler {
	return func(event *dto.WSPayload, data *dto.WSDirectMessageData) error {
		if data != nil {
			fmt.Printf("received a direct message in guild channel %s: %s\n", sdklog.SafeText(data.ChannelID), sdklog.SafeText(data.Content))
		}
		return nil
	}
}

// GuildMessageHandler 处理消息事件
func GuildMessageHandler() event.MessageEventHandler {
	return func(event *dto.WSPayload, data *dto.WSMessageData) error {
		if data != nil {
			fmt.Printf("received a message in channel %s: %s\n", sdklog.SafeText(data.ChannelID), sdklog.SafeText(data.Content))
		}
		return nil
	}
}
