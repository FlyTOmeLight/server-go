# server-go 项目规则

## 项目定位

本仓库维护可供其他 Go 项目引用的 Gin 服务框架。公开模块路径为 `github.com/FlyTOmeLight/server-go`。搭建业务项目时先读 [README.md](README.md)，从 `examples/minimal-service` 打通 HTTP 流程；需要数据库、SDK 和 CLI 时参考 `examples/example-hello-world`。

## 业务项目组织

新建业务服务、调整目录或编写 SDK 时，读取 [Go 项目组织约定](docs/go-project-layout.md) 的对应章节。采用 `cmd` + `internal`，按 Controller、Service、Table、SDK 的实际职责组织；框架接口和配置路径以当前实现为准。

参考其他业务项目时，只沉淀可复用的工程规则；本仓库文档不保留参考业务项目的名称、本地路径或专有实现细节。

## 框架与业务边界

- 框架根模块只放跨项目复用的能力。业务项目使用自己的 module path，把领域逻辑放在自己的 `internal/` 中，通过发布版本引用本框架。
- 启动入口显式构造配置和日志，再用 `server.NewBootstrap` 初始化所需资源模块；失败回滚，退出逆序关闭。HTTP 服务通过 `host.NewHTTPHost` 装配 `host.Service`。`RegisterRoutes` 只安装路由，`Start(ctx)` 启动所属任务，`Stop(ctx)` 取消并等待任务。进程入口处理信号，调用 `Run(ctx)`，并安排 `Close`。
- 配置使用 `conf.Manager` 实例和独立快照；基础设施配置由装配代码从快照解析，资源通过构造函数注入。配置路径、占位符和动态更新范围见 [conf/README.md](conf/README.md)。
- HTTP 参数、响应和错误沿用框架现有契约。修改前检查 `pkg/gin.go`、`response.go`、`errors/errors.go` 及相关 SDK 调用方。
- 日志使用仓库内 `logger` 包，复用 Zap 和 Lumberjack。修改日志时保留请求 ID、级别过滤与文件轮转。许可证中间件依赖 `middleware.LicenseAuthorizer`；客户端创建、启动和关闭由业务项目负责。
- 数据库仅使用公开驱动。新增驱动同时更新配置检查、数据库装配、类型映射和相关测试。
- 幂等仅在鉴权和本次授权之后启用；`middleware.Idempotent` 作为需要幂等的路由的最后一个 handler。控制器归业务服务所有，不在路由重建时新建。

## 修改与验证

- 根模块与四个带 `go.mod` 的示例是独立模块。修改模块路径或依赖时，同步检查示例的 `require`、本地 `replace`、Makefile 的链接变量和 `scripts/gen_code.py`。
- 代码生成器以 `examples/example-hello-world` 为模板。修改模板或生成器后，在临时目录生成独立项目，执行 `go mod tidy` 并构建服务端与 CLI。
- Go 代码修改后执行 `gorgeous ./...`，并用 `gorgeous -d ./...` 检查格式，以符合推送钩子的要求。在受影响模块执行 `go test ./...`、`go vet ./...`。根模块测试需要 `CGO_ENABLED=1` 以使用 SQLite；共享状态和生命周期改动对受影响包执行 `go test -race`。
- 跨模块改动必须检查根模块和四个示例。最小服务应能启动，并通过 `/hello` 请求返回 `{"message":"hello"}`。
- 发布文件只包含公开依赖、示例值和环境变量占位符。代码索引、运行日志、数据库和本地构建产物由 `.gitignore` 排除。

## 公开版本边界

公开版本从清理后的代码快照建立历史。旧远端与旧 Git 历史保留在本地，不推送到公开仓库。旧版本中的私有 `logger-go` 已替换为本仓库 `logger` 包；私有许可证 SDK 改为调用方注入；依赖私有驱动的达梦数据库支持已移除。
