# QQ 机器人示例

`native-webhook` 演示 QQ 原生事件接收，是当前接入示例。

`receive-and-send` 演示消息收发与云函数部署；`custom-filter` 演示请求过滤器；`custom-logger` 演示日志接入；`simulate-callback-request` 演示回调请求模拟。

统一回归测试位于主模块，在仓库根目录执行 `go test ./...`。CI 同时编译本目录中的示例。
