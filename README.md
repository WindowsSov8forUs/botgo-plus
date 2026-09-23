# botgo-plus

QQ 官方机器人平台的 Go 原生通信 SDK，基于 `tencent-connect/botgo` 的独立 fork。这里不是腾讯官方发行版本。

本项目维护鉴权、HTTP API、Webhook / WebSocket、QQ 原生事件和媒体上传；不负责上层协议转换或消息渲染。

当前开发分支要求 **Go 1.23+**（由 Resty v2.17.2 / x/net v0.43.0 的依赖声明确定），生产环境应使用 Go 官方仍维护的工具链及安全补丁。本 fork 包含模块路径和部分数据类型的破坏性变更，使用方式见下文。

## 已实现的通信能力

| 模块 | 内容 |
| --- | --- |
| Token / HTTP | 每 App 独立缓存、singleflight、显式失效、一次受控鉴权重试、同源凭证保护、结构化错误和请求级 TraceID |
| Webhook | 完整限长读取、Ed25519、独立挑战应答、处理成功后 ACK、失败状态码、每 App 独立分发器 |
| QQ 原生消息 | 新 OpenID / 角色 / 卡片 / 嵌套元素 / 引用索引 / 语音字段，保留完整原始载荷 |
| 消息发送 | 原生 Markdown / Ark / Keyboard、C2C 专用流式接口、独立互动响应；回复凭据与序号由调用方填写 |
| 媒体 | URL 上传、预上传、受限并发的分片 PUT、确认、合并；Uploader 不自动发送，低层请求仅在显式设置 srv_send_msg=true 时请求发送 |
| QQ 群 | 资料、成员详情和分页、批量移除、成员禁言；与 QQ 频道接口分开 |
| WebSocket | 每分片会话、串行写、ACK 超时检测、快照状态和受控重连 |

存在 SDK 方法不代表账号已获平台授权。真实 QQ 权限、频控、媒体限制和特定接入方式需要使用自己的测试机器人验证。历史频道功能与 Redis 多机管理器保留，但未在本轮逐项做生产联调。

## 安装与迁移

新的模块路径：

```text
github.com/WindowsSov8forUs/botgo-plus
```

包名仍为 `botgo`。尚未发布对应版本时，下游可使用本地替换：

```go
require github.com/WindowsSov8forUs/botgo-plus v0.0.0
replace github.com/WindowsSov8forUs/botgo-plus => ../botgo-plus
```

重要变更：`file_info` 改为不透明字符串；旧 `MessageToCreate.Stream` 保留原样透传，但不等同于新流式接口，也不保证平台仍接受；平台失败与待审核分别返回；网络和解析错误保留底层错误，不据此推断消息是否已送达；Webhook 对无效签名、应用 ID 和超限载荷显式拒绝。

## 最小客户端

构造客户端不会发送消息。Token 在实际调用时按需获取：

```go
import (
    botgo "github.com/WindowsSov8forUs/botgo-plus"
    "github.com/WindowsSov8forUs/botgo-plus/token"
)

credentials := &token.QQBotCredentials{AppID: appID, AppSecret: appSecret}
source := token.NewQQBotTokenSource(credentials)
client, err := botgo.NewClient(appID, source)
if err != nil {
    return err
}
```

Token 统一通过 `NewQQBotTokenSource(credentials, options...)` 构造；省略选项使用默认配置，需要自定义端点、HTTP 客户端或超时时直接传入对应选项。返回值可直接调用 `TokenContext`、`Invalidate`，同时实现 `oauth2.TokenSource`。

定时刷新按 Token 实际剩余有效时间调度，保留上游提前量和随机抖动；缓存使用标准 `oauth2.Token.Valid()`。普通消息及流式消息的业务字段由 QQ 平台校验，SDK 不维护额外的 DTO 业务校验层，也不自动更改请求或序号。

新增方法可直接通过 `client` 调用。旧 `NewOpenAPI` 和大接口保留兼容入口，但推荐 `NewClient` 处理配置错误及访问新增原生能力。

## 接收原生事件

[examples/native-webhook](examples/native-webhook/main.go) 提供可运行的接收示例：读取 `QQBOT_APP_ID`、`QQBOT_APP_SECRET`，默认监听 `127.0.0.1:8080/qqbot`，置于 HTTPS 反向代理后使用。

```text
go -C examples run ./native-webhook
```

示例只记录事件元数据，不自动发消息或修改群。生产使用时，业务处理完成或持久队列确认接收后才返回成功；应用应自行实现幂等、重复投递处理与审核结果关联。不要把打印日志当作可靠队列。

WebSocket 回调返回错误时记录错误并继续分发，不用网关重连代替业务重试；恢复序号在回调处理后推进。队列采用上游容量的有界背压，应用不能将它当作持久队列，长期阻塞仍可能影响连接。

原始 JSON 可能包含用户消息、标识和敏感扩展字段，禁止默认完整输出到日志。

## 媒体上传

`media.NewUploader(client, config)` 接受稳定、可并发读取的 `io.ReaderAt`。它执行校验和、预上传、分片上传和合并，返回原始 `file_info`，不会替调用方发送消息。

默认并发 4、文件上限 256 MiB、PUT 最多 3 次是本地资源保护值，不是 QQ 平台配额。正式预签名 URL 要求 HTTPS。取消、分片失败会返回错误，不执行无界重试。网络错误不能证明平台未执行请求，SDK 不自动重发聊天消息。

## 开发与测试

在仓库根目录运行：

```text
go test ./...
```

测试按包集中维护，覆盖 QQ 请求与响应、Token 获取与刷新、原生事件分发、Webhook 签名与应答、WebSocket 帧和媒体分片上传。网络用例使用本地假服务。

CI 采用单一 Linux / stable Go 任务，执行同一测试命令并构建 examples。旧的真实 QQ 账号测试、独立 Redis 集成测试和重复平台/版本矩阵已移除。CodeQL 安全扫描保持独立。真实账号权限、限流与服务端行为需要另行联调，不能由本地回归结果推定。

## 协议来源与归属

以 [QQ 官方机器人文档](https://bot.q.qq.com/wiki/develop/api-v2/) 为协议契约。

保留上游 [LICENSE](LICENSE) 和作者归属。
