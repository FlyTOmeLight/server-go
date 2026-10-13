# 可运行示例

所有 HTTP 示例以 [example-hello-world](example-hello-world/README.md) 为目录与工程约定基准：`cmd/server` 处理信号和退出码，`service` 公开服务构造并装配资源，`internal/app/<领域>` 放 Controller 与业务 Service，数据库操作放 `internal/table`，对外 SDK 放 `sdk/go/<领域>v1`。只创建实际需要的层。

| 示例 | 内容 | 模块 |
| --- | --- | --- |
| [minimal-service](minimal-service/README.md) | `/hello`，最少 HTTP 组件 | 根模块 |
| [example-hello-world](example-hello-world/README.md) | SQLite CRUD、SDK、CLI、生成模板 | 独立模块 |
| [example-service1](example-service1/README.md) | REST 数据集 CRUD、SDK、CLI | 独立模块 |
| [example-service2](example-service2/README.md) | 可组合的无状态 HTTP 服务 | 独立模块 |
| [example-combine-service](example-combine-service/README.md) | 组合三个公开服务并共享数据库 | 独立模块 |
| [streaming-service](streaming-service/README.md) | SSE、取消传播与流式 SDK | 根模块 |
| [postgresql-service](postgresql-service/README.md) | PostgreSQL CRUD、请求 tag 校验、默认值、分页与 SDK | 根模块 |
| [example-leader-election](example-leader-election/README.md) | 独立 worker 与 leader 任务 | 根模块 |

从各示例目录运行 `make run`；PostgreSQL 示例默认监听 `127.0.0.1:18080`，其他 HTTP 示例默认监听 `127.0.0.1:8080`，单独运行时应避免端口冲突。SQLite 示例需要 CGO 和 C 编译器。`make build` 输出到 `output/bin`，`make package` 保留 `output/conf/server/config.toml`；在 `output` 目录启动打包后的服务。`make check` 检查格式、测试和 vet，完整跨模块与生成器验证在仓库根目录运行 `make check`。

HTTP 配置默认从当前目录的 `conf/server/config.toml` 读取，也可以通过 `SERVER_CONFIG_PATH` 指定文件。`app.Run` 只解析一次业务配置并管理资源关闭；`AutoMigrate` 由装配代码显式传入服务构造函数。Controller 与对应业务 Service 使用 `*core.Context`，存储、SDK 和后台任务使用标准 `context.Context`。

普通 HTTP 接口在 Controller 用 `serrors.AbortWithError(ctx, err)` 完成错误响应并返回，业务 Service 只返回错误。静态默认值在绑定前设置，请求校验复用 `pkg/binder`。SDK 的普通 JSON 请求用 `sdk.Complete` 处理完成结果；CLI 通过执行 context 获取客户端和传递取消。这些写法在 HelloWorld、Dataset 和 PostgreSQL 示例中保持一致，生成器沿用 HelloWorld 模板。SSE 已开始输出后的错误使用流式事件，独立 worker 按任务生命周期退出；具体请求协议和业务职责按各示例保留。

独立模块用本地 `replace` 引用当前框架源码；根模块示例无需额外的 `go.mod`。`app` 入口从 v0.5.0 开始提供；复制为独立项目时同步修改 import 与 module path，并引用当前发布版 `github.com/FlyTOmeLight/server-go@v0.9.0`。本地替换用于联合开发验证。

每次修改框架后，在根目录运行 `make check`。它使用 Docker 启动隔离的 PostgreSQL，并实际运行全部 HTTP 示例、CLI、worker 与生成项目；所有容器和临时项目在检查结束后清理。Docker 不可用会明确失败。PostgreSQL 示例的单独启动方式见其 README。

所有业务 HTTP handler 都维护 Swagger 注释，覆盖方法、路径、参数、成功响应和重要错误。每个 HTTP 示例均支持 `make install-doc` 和 `make doc`：HelloWorld、Dataset 保留现有 Go/JSON/YAML 产物，其他示例输出 JSON/YAML 到 `doc/openapi`。组合示例的 `make doc` 更新三个子服务文档，按各自 BasePath 使用；独立 worker 没有 HTTP 接口。新 API 的资源路由建议和注释要求见 [HTTP 协议与 Swagger](../docs/go-project-layout.md#http-协议与-swagger)。
