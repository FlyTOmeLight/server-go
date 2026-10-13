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
| [example-leader-election](example-leader-election/README.md) | 独立 worker 与 leader 任务 | 根模块 |

从各示例目录运行 `make run`；HTTP 示例默认监听 `127.0.0.1:8080`，单独运行时应避免端口冲突。SQLite 示例需要 CGO 和 C 编译器。`make build` 输出到 `output/bin`，`make package` 保留 `output/conf/server/config.toml`；在 `output` 目录启动打包后的服务。`make check` 检查格式、测试和 vet，完整跨模块与生成器验证在仓库根目录运行 `make check`。

HTTP 配置默认从当前目录的 `conf/server/config.toml` 读取，也可以通过 `SERVER_CONFIG_PATH` 指定文件。`app.Run` 只解析一次业务配置并管理资源关闭；`AutoMigrate` 由装配代码显式传入服务构造函数。Controller 与对应业务 Service 使用 `*core.Context`，存储、SDK 和后台任务使用标准 `context.Context`。

独立模块用本地 `replace` 引用当前框架源码；根模块示例无需额外的 `go.mod`。`app` 入口从 v0.5.0 开始提供；复制为独立项目时同步修改 import 与 module path，并引用 `github.com/FlyTOmeLight/server-go@v0.5.0`。本地替换用于联合开发验证。
