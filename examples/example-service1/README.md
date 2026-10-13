# 数据集服务

REST 风格的 SQLite CRUD 示例，具有 Go SDK 和 CLI。独立模块通过本地 `replace` 使用当前框架。从本目录运行：

```sh
go mod tidy
make run
```

另一终端创建和读取数据集：

```sh
curl -H 'Content-Type: application/json' -d '{"localName":"sample","description":"example","fileFormat":"File"}' \
  http://127.0.0.1:8080/v1/projects/project1/datasets
curl http://127.0.0.1:8080/v1/projects/project1/datasets/sample
```

`cmd/server` 调用 `app.Run`，`service.Build` 初始化数据库，`service.NewExampleService(ctx, database, autoMigrate)` 支持组合应用注入共享资源。`internal/app/dataset/controller.go` 绑定参数和响应，`service.go` 处理业务，`internal/table/dataset.go` 负责查询。迁移开关由配置中的 `DB.AutoMigrate` 控制。

`sdk/go/trainv1` 的所有 HTTP 方法接收标准 `context.Context`，向请求传递取消和截止时间，并使用 `sdk.Complete` 统一处理请求完成，保留传输错误、取消和 HTTP 失败状态。服务端 Controller 和业务 Service 使用 `*core.Context`，向数据库传递 `ctx.Request.Context()`。

构建并使用 CLI：

```sh
make build
./output/bin/example1 config set core.endpoint=http://127.0.0.1:8080
./output/bin/example1 dataset create --project-name=project1 --local-name=another --file-format=File
./output/bin/example1 dataset list --project-name=project1
```

SDK 便捷构造使用 `trainv1.NewClient(endpoint)`；需要 API Key 或其他传输选项时，传入 `sdk.Config` 调用 `NewClientWithConfig`，调用结束后关闭客户端。CLI 通过 `cli.Client(cmd.Context())` 获取当前执行的客户端，Apply 注册归当前命令行实例，退出时统一释放资源。

仅保留真实参与请求的参数。`make doc` 重建 Swagger，`make install-doc` 安装文档工具；`make check` 验证格式、SDK CRUD 和 vet。配置、打包与版本约定见 [示例索引](../README.md) 和 [项目组织约定](../../docs/go-project-layout.md)。
