# 无状态服务

展示不需要数据库的可组合 HTTP 服务。从本目录运行：

```sh
go mod tidy
make run
curl http://127.0.0.1:8080/service2/hello
# {"message":"hello from service2"}
```

`cmd/server` 调用 `app.Run`，`service.Build` 装配服务；组合项目导入 `service.NewExampleService()`。HTTP 逻辑在 `internal/app/hello/controller.go`。静态响应无需空业务 Service、Table 或 SDK。

原来的 job CLI 只有空实现，已删除；本示例通过真实 HTTP 请求展示服务组合。`make check` 验证格式、测试和 vet，配置与打包约定见 [示例索引](../README.md)。
