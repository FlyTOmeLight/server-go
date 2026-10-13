# 最小 HTTP 服务

不需要数据库或外部服务。从本目录运行：

```sh
make run
curl http://127.0.0.1:8080/hello
# {"message":"hello"}
```

`cmd/server/server.go` 处理信号，`service/build.go` 通过 `app.Run` 装配公开服务，`internal/app/hello/controller.go` 返回静态响应。这种接口无需增加空业务 Service、Table 或 SDK。配置在 `conf/server/config.toml`，支持 `SERVER_CONFIG_PATH`。Ctrl+C 或 SIGTERM 会排空 HTTP 并退出。

本示例属于框架根模块。`make build` 构建到 `output/bin`，`make package` 复制配置；在本目录执行 `make check`，完整验证见 [示例索引](../README.md)。
