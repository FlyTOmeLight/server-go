# HelloWorld

这是包含 SQLite CRUD、Go SDK 和 CLI 的服务模板。入口在 `cmd/server`，公开服务构造与依赖装配在 `service`，业务接口在 `internal/app/module`。数据库默认使用本地 SQLite，需要 C 编译器和 `CGO_ENABLED=1`。

```sh
go mod tidy
CGO_ENABLED=1 go run ./cmd/server
```

另一终端请求示例：

```sh
curl -H 'Content-Type: application/json' \
  -d '{"name":"example","projectID":"project1"}' \
  http://127.0.0.1:8080/helloworld/internal/v1/CreateModule
```

入口通过 `app.Run` 统一处理初始化、配置监听和资源清理。`service.Build` 装配独立服务；组合项目可以导入 `service.NewHelloWorldService`，注入自己的共享数据库。该应用入口从 v0.5.0 开始提供，示例通过本地 `replace` 联合验证当前框架源码。

配置路径可用 `SERVER_CONFIG_PATH` 指定。当前示例采用 Fake 身份，在真实业务中由应用装配认证与授权。

## 新增业务接口

以现有 `GetModule` 为参考，按以下路径完成一个接口：

1. 在 `sdk/go/helloworldv1/helloworld_api_module.go` 定义请求与响应。明确 JSON 字段、校验和更新字段的省略语义。
2. 在 `internal/table/module.go` 增加查询；接收 `context.Context` 和数据库句柄。业务权限、事务与默认值放在 `internal/app/module/service.go`。
3. 在 `controller.go` 绑定参数、调用业务 Service 并返回响应。Controller 与业务 Service 传递 `*core.Context`，数据库与 SDK 调用使用 `ctx.Request.Context()`；同步注册路由与 Swagger 注释。控制器在服务构造时创建，路由重建复用同一实例。
4. 在 SDK 的 API 接口与客户端方法中新增调用；方法接收 `context.Context`，每次请求调用 `SetContext(ctx)`。失败必须保留 HTTP 状态与服务错误，非 JSON 错误也返回可检查的错误。
5. 如有 CLI，使用 `cmd.Context()` 调用 SDK。根据操作契约选择是否启用幂等，先完成鉴权和本次授权。
6. 在 `service/module_sdk_test.go` 增加实际 HTTP 用例，验证业务结果；在 SDK 测试中覆盖该操作新增的重要失败路径。

重新生成接口文档：

```sh
make install-doc
make doc
```

格式化后执行完整项目检查：

```sh
gorgeous ./...
make check
```

`make check` 检查格式、运行测试和 vet，并对服务与 SDK 执行竞态检测。Go、C 编译器、make 与 gorgeous 需要预先安装；gorgeous 从 [FlyTOmeLight/go_fmt](https://github.com/FlyTOmeLight/go_fmt) v0.9.0 的仓库源码安装，步骤见[框架开发验证](https://github.com/FlyTOmeLight/server-go#开发验证)。收到 SIGTERM 或 Ctrl+C 后，服务排空 HTTP 并关闭资源。

## 构建与 CLI

```sh
make build
./output/bin/helloworld config set core.endpoint=http://127.0.0.1:8080
./output/bin/helloworld module create --name=example --project-id=project1
./output/bin/helloworld module list
```

`make package` 将二进制放在 `output/bin`，配置放在 `output/conf/server`。从 `output` 目录运行 `./bin/helloworld-server`。配置只声明实际初始化的数据库；共通规则与全部示例见 [示例索引](../README.md)。
