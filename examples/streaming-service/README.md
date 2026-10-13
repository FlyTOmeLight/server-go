# 流式服务

这是根模块中的可运行示例，不需要数据库、模型账户或外部服务。`internal/greeting` 用本地数据源逐步发送文本；将 `GenerateGreeting` 换成接收 context 的模型调用即可接入真实上游。

```sh
cd examples/streaming-service
go run ./cmd/server
```

另一终端读取事件：

```sh
curl -N 'http://127.0.0.1:8080/stream?name=world'
```

参数 `name` 默认是 `world`，最多 64 个字符。事件为 `message`（JSON 数据 `{"text":"..."}`）、`done`（正常结束）或 `error`（框架错误结构）。看到 `done` 才表示完整成功；提前 EOF 表示流中断。连接开始前的参数错误使用正常 HTTP 错误响应，开始发送后不追加 JSON 响应或重写 HTTP 状态。

## 生命周期与时限

业务操作最多运行 30 秒，每次发送最多等待 5 秒。`stream.NewSSE` 替换当前路由的总写入期限，不改变普通 JSON 路由的服务器超时。调试日志最多采集 8 KiB 响应，Console 代理识别带 charset 的 SSE。

生成函数在请求 goroutine 中执行，遵守 context，并在发送失败时返回。客户端关闭连接会取消生成；业务实现若创建自己的 goroutine，应在函数返回前取消并等待它。SSE 路由不使用整段响应幂等缓存。框架退出会先排空 HTTP，再取消剩余请求。

`sdk/greeting.Client.Open(ctx, name)` 直接返回 `io.ReadCloser`，客户端应在读完或提前停止时关闭它，也可取消传入的 context。SDK 使用独立的 35 秒总时限；HTTP 失败由 SDK 关闭响应体。流中途的错误由调用方按 SSE 事件处理。

```go
client, err := greeting.NewClient("http://127.0.0.1:8080")
if err != nil { return err }
body, err := client.Open(ctx, "world")
if err != nil { return err }
defer body.Close()
// 使用项目选定的 SSE 客户端解码事件；body 保持流式读取。
```

将示例复制成独立业务项目时，建立自己的 go.mod，修改本示例 import 路径，并通过 `go get github.com/FlyTOmeLight/server-go@v0.3.0` 引用框架。`stream` 包从 v0.3.0 开始提供。

在仓库根目录执行 `make check`，会验证真实网络上的首事件、写入期限、SDK 关闭取消、Console 透传、流内错误以及示例进程启动与退出。
