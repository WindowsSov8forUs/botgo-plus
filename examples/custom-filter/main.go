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
	"github.com/google/uuid"
	"gopkg.in/yaml.v3"

	"github.com/WindowsSov8forUs/botgo-plus/constant"
	"github.com/WindowsSov8forUs/botgo-plus/openapi"
	"github.com/WindowsSov8forUs/botgo-plus/token"
)

const (
	host_ = "0.0.0.0"
	port_ = 9000
	path_ = "/qqbot"
)

func main() {
	openapi.RegisterReqFilter("set-trace", ReqFilter)
	openapi.RegisterRespFilter("get-trace", RespFilter)
	// 加载 appid 和 token
	content, err := os.ReadFile("config.yaml")
	if err != nil {
		log.Fatalln("load config file failed, err:", err)
	}
	credentials := &token.QQBotCredentials{}
	if err = yaml.Unmarshal(content, &credentials); err != nil {
		log.Fatalln("parse config failed, err:", err)
	}
	tokenSource := token.NewQQBotTokenSource(credentials)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel() //释放刷新协程
	if err = token.StartRefreshAccessToken(ctx, tokenSource); err != nil {
		log.Fatalf("start access token refresh failed: %s", sdklog.SafeError(err))
	}
	// 初始化 openapi，正式环境
	api := botgo.NewOpenAPI(credentials.AppID, tokenSource).WithTimeout(5 * time.Second).SetDebug(true)
	// 根据不同的回调，生成 intents
	_ = event.RegisterHandlers(GuildATMessageEventHandler(api))
	// 初始化 openapi，正式环境
	http.HandleFunc(path_, func(writer http.ResponseWriter, request *http.Request) {
		webhook.HTTPHandler(writer, request, credentials)
	})
	if err = http.ListenAndServe(fmt.Sprintf("%s:%d", host_, port_), nil); err != nil {
		log.Fatalf("HTTP server stopped: %s", sdklog.SafeError(err))
	}
}

// ReqFilter 自定义请求过滤器
func ReqFilter(req *http.Request, _ *http.Response) error {
	req.Header.Set("X-Custom-TraceID", uuid.NewString())
	return nil
}

// RespFilter 自定义响应过滤器
func RespFilter(req *http.Request, resp *http.Response) error {
	log.Printf("request filter added trace ID %s", sdklog.SafeText(req.Header.Get("X-Custom-TraceID")))
	log.Printf("OpenAPI returned trace ID %s", sdklog.SafeText(resp.Header.Get(constant.HeaderTraceID)))
	return nil
}

// GuildATMessageEventHandler 实现处理 at 消息的回调
func GuildATMessageEventHandler(api openapi.OpenAPI) event.ATMessageEventHandler {
	return func(event *dto.WSPayload, data *dto.WSATMessageData) error {
		log.Printf("received %s message: %s", sdklog.SafeText(string(event.Type)), sdklog.SafeText(data.Content))
		input := strings.ToLower(message.ETLInput(data.Content))
		log.Printf("cleaned input content is: %s", sdklog.SafeText(input))
		return nil
	}
}
