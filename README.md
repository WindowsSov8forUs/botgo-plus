# BotGo Plus

基于 `github.com/tencent-connect/botgo` 的增强版 SDK。

本库最初作为 `WindowsSov8forUs/GlycCat` 的内部库使用，现已独立维护，并补充了富媒体格式适配能力（image/mp4/silk）。

## 改动来源

本项目中的以下能力，来源于 `WindowsSov8forUs/GlycCat`（`pkg/botgo`）的内部实现沉淀：

- OpenAPI `v1` + `v2` 双版本实现
- `WebhookManager` 与 webhook server 实现
- 新版 token 结构（`token.BotToken(appID, appSecret, token, token.TypeQQBot)`）
- 本地/多实例会话管理能力（默认本地，不依赖 Redis）

## 快速开始

### 1. 创建 Token 与 OpenAPI 客户端

```go
package main

import (
    "context"
    "log"
    "time"

    "github.com/tencent-connect/botgo"
    "github.com/tencent-connect/botgo/openapi"
    "github.com/tencent-connect/botgo/token"
)

func main() {
    ctx := context.Background()

    tk := token.BotToken(
        1234567890,          // appID
        "app-secret",       // appSecret
        "bot-token",        // bot token
        token.TypeQQBot,
    )
    if err := tk.InitToken(ctx); err != nil {
        log.Fatal(err)
    }

    if err := botgo.SelectOpenAPIVersion(openapi.APIv2); err != nil {
        log.Fatal(err)
    }

    api := botgo.NewOpenAPI(tk).WithTimeout(10 * time.Second)
    me, err := api.Me(ctx)
    if err != nil {
        log.Fatal(err)
    }
    log.Printf("bot user: %+v", me)
}
```

### 2. WebSocket 模式

```go
package main

import (
    "context"
    "log"

    "github.com/tencent-connect/botgo"
    "github.com/tencent-connect/botgo/dto"
    "github.com/tencent-connect/botgo/event"
    "github.com/tencent-connect/botgo/token"
    "github.com/tencent-connect/botgo/websocket"
)

func main() {
    ctx := context.Background()
    tk := token.BotToken(1234567890, "app-secret", "bot-token", token.TypeQQBot)
    _ = tk.InitToken(ctx)

    api := botgo.NewOpenAPI(tk)
    wsInfo, err := api.WS(ctx, nil, "")
    if err != nil {
        log.Fatal(err)
    }

    var atHandler event.ATMessageEventHandler = func(evt *dto.Payload, data *dto.ATMessageData) error {
        log.Printf("AT message: %s", data.Content)
        return nil
    }

    intent := websocket.RegisterHandlers(atHandler)
    if err := botgo.NewSessionManager().Start(wsInfo, tk, &intent); err != nil {
        log.Fatal(err)
    }
}
```

## Webhook 使用（重点）

`WebhookManager.Start` 是阻塞调用，通常建议在 goroutine 中启动。

```go
package main

import (
    "log"

    "github.com/tencent-connect/botgo"
    "github.com/tencent-connect/botgo/dto"
    "github.com/tencent-connect/botgo/event"
    "github.com/tencent-connect/botgo/webhook"
)

func main() {
    var groupHandler event.GroupATMessageEventHandler = func(evt *dto.Payload, data *dto.GroupATMessageData) error {
        log.Printf("group at message: %s", data.Content)
        return nil
    }

    // 兼容入口，内部等价于 event.RegisterHandlers(...)
    webhook.RegisterHandlers(groupHandler)

    cfg := &dto.Config{
        Host:      "0.0.0.0",
        Port:      9000,
        Path:      "/qqbot/callback",
        AppId:     1234567890,
        BotSecret: "app-secret",
    }

    go func() {
        if err := botgo.NewWebhookManager().Start(cfg); err != nil {
            log.Printf("webhook stopped: %v", err)
        }
    }()

    select {}
}
```

### Webhook 配置要点

- 必须先注册事件处理器，再启动 `WebhookManager`。
- 平台配置的回调地址要与 `Host/Port/Path` 对应。
- 对外建议通过 HTTPS 反向代理暴露 webhook（SDK 内置 HTTP 服务，不直接处理 TLS 证书）。
- SDK 会校验回调签名和 `X-Bot-Appid`。

## 富媒体格式适配扩展

本仓库新增了文件类型适配层，已接入：

- `openapi/v1` 和 `openapi/v2` 的 `PostGroupMessage` / `PostC2CMessage`

行为说明：

- 仅当消息类型为 `RichMediaMessage` 且包含 `file_data` 时触发。
- 按 `file_type` 自动适配：
- `1` 图片：转为平台支持的图片格式（png/jpg/gif）
- `2` 视频：转为 mp4（H264/AAC）
- `3` 语音：转为 silk（已支持 amr/silk 直传）
- 若源文件已是支持格式，不会重复转换。

依赖说明：

- 视频/音频转换依赖本机 `ffmpeg`
- silk 编解码器位于 `pkg/silk/exec/`

## Redis 说明

- 默认 `SessionManager` 为本地实现，不依赖 Redis。
- Redis 仅用于 `sessions/remote` 分布式场景。
- Redis 相关测试默认跳过；设置 `BOTGO_REDIS_TEST=1` 才会执行。

## 开发

- 开发说明见 [DEVELOP.md](./DEVELOP.md)
- 基础自测命令：

```bash
go test ./...
```
