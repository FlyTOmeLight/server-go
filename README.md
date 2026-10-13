# server-go

基于 Gin 的 Go 服务框架，提供 HTTP 服务生命周期、配置加载与热更新、通用中间件、数据库连接、SDK 和 CLI 能力。

模块路径：`github.com/FlyTOmeLight/server-go`。当前工作树需要 Go 1.27 或更高版本。使用 SQLite 的项目需要启用 CGO，并安装 C 编译器。所有 Go 依赖均来自公开模块。

## 快速启动

最小示例不需要数据库、Redis 或外部鉴权服务：

```sh
git clone https://github.com/FlyTOmeLight/server-go.git
cd server-go/examples/minimal-service
go run ./cmd/server
```

在另一个终端验证：

```sh
curl http://127.0.0.1:8080/hello
# {"message":"hello"}
```

按 Ctrl+C 停止服务。`/ping`、`/healthz` 和 `/version` 由框架提供。示例使用 `Fake` 鉴权；业务项目应按自身认证需求配置鉴权。

## 搭建项目

新项目先复制 `examples/minimal-service` 中的 `cmd/`、`service/`、`internal/` 和 `conf/`，同步修改示例 import 为自己的模块路径。示例使用 v0.5.0 提供的 `app` API：

```sh
go mod init github.com/your-org/your-service
go get github.com/FlyTOmeLight/server-go@v0.5.0
go mod tidy
go run ./cmd/server
```

修改配置中的 `AppName`、`AppCode` 和监听地址。业务入口放在 `cmd/<程序名>/`，公开服务构造与类型化装配放在同级 `service/`，私有业务逻辑放在 `internal/app/<领域>/`。框架默认从工作目录读取 `conf/server/config.toml`，也可设置 `SERVER_CONFIG_PATH`。

按需要选择可运行示例：

| 示例 | 适用场景 |
| --- | --- |
| [minimal-service](examples/minimal-service/README.md) | 只需要 HTTP 和配置 |
| [example-hello-world](examples/example-hello-world/README.md) | SQLite CRUD、Go SDK 与 CLI；生成器使用此模板 |
| [example-service1](examples/example-service1/README.md) | REST 数据集 CRUD、SDK 与 CLI |
| [example-service2](examples/example-service2/README.md) | 无状态 HTTP 服务与公开构造函数 |
| [example-combine-service](examples/example-combine-service/README.md) | 组合三个服务、共享 SQLite 与组合 CLI |
| [streaming-service](examples/streaming-service/README.md) | SSE、取消传播、流式 SDK 和分段写入期限 |
| [example-leader-election](examples/example-leader-election/README.md) | 独立 worker 与 leader 任务 |

所有示例遵循同一套目录、配置、context 和生命周期约定，详见 [示例索引](examples/README.md) 与 [Go 项目组织约定](docs/go-project-layout.md)。HTTP 示例从各自目录执行 `make run`、`make build` 和 `make check`，打包配置统一保留 `conf/server/`。

也可从仓库根目录使用现有生成器：

```sh
python3 scripts/gen_code.py \
  --name MyService \
  --modules Item \
  --go-mod github.com/your-org/my-service \
  --output ../generated
cd ../generated/my-service
go mod tidy
go build ./cmd/server ./cmd/cli
```

生成器使用 HelloWorld 模板，并引用发布版框架 v0.5.0。按项目需求调整配置，API 文档可通过示例 Makefile 的 `doc` 目标重新生成。

## 当前工作树的依赖与 HTTP/2

Kubernetes 组件使用 v0.37.1；`kube-openapi` 固定为它们声明的 `v0.0.0-20260721132016-d427ff9ee9ad`，保持 schema v6 类型一致，不单独追逐该模块的最新提交。

`HTTPServer.UseH2C` 通过标准库 `http.Server.Protocols` 同时启用 HTTP/1 和明文 HTTP/2，退出由同一个 Server 排空请求并关闭连接。明文 HTTP/2 客户端使用 prior knowledge 模式，旧的 HTTP/1 Upgrade 升级方式不再使用。

## 应用启动（v0.5.0）

`app.Run(ctx, options, build)` 统一创建配置与日志、限制初始化时间、监听配置变化、运行 HTTPHost，并在退出或启动失败时清理资源。进程入口仍负责退出信号与退出码。

业务先声明配置结构体，字段名对应 TOML 节，名称不同时通过 `mapstructure` tag 指定 key；`app.Run` 从同一份配置快照一次解析全部字段，统一返回解析错误。装配函数接收初始化 context、`*app.Runtime` 和类型化配置指针，通过 `runtime.Initialize(ctx, modules...)` 初始化需要的资源，再通过 `runtime.AddService(svc)` 交付构造好的服务。资源按交付顺序逆序关闭；服务即使尚未启动，也会调用一次 `Stop`。构造失败前尚未交付的部分资源仍由业务构造函数清理。

`app.Options` 可指定配置路径、日志配置、HTTP setup 和初始化/清理期限，默认初始化 15 秒、每次应用清理操作 5 秒。HTTP 请求排空和运行中服务停止的期限仍由 HTTPHost 管理。初始化 context 不用于启动长期任务；任务通过 `host.Service.Start(ctx)` 获取运行 context。

例如 HelloWorld 声明以下配置，装配函数直接使用 `cfg.DB`。业务配置是启动时的独立值，配置热更新不会原地修改它。

```go
type Config struct {
    DB db.Config
}

func Build(ctx context.Context, runtime *app.Runtime, cfg *Config) error {
    database := &db.Module{Config: &cfg.DB}
    if err := runtime.Initialize(ctx, database); err != nil {
        return err
    }
    svc, err := NewHelloWorldService(ctx, database.DB, cfg.DB.AutoMigrate)
    if err != nil {
        return err
    }
    return runtime.AddService(svc)
}
```

数据库 SQL 日志统一通过框架 logger 输出：普通查询为 Info，超过 200 毫秒的慢查询为 Warn，失败查询为 Error，并遵守日志实例的级别过滤。RequestID 中间件将请求 ID 写入标准请求 context，GORM 日志适配器从每次操作的 context 读取日志实例和 Request ID。初始化与迁移使用应用日志实例。

HTTP Controller 和对应业务 Service 使用 `*core.Context`；数据库和 SDK 操作传递 `ctx.Request.Context()`，保留取消、超时和 Request ID，避免数据库后台任务读取被 Gin 复用的对象。

可组合的服务放在与 `cmd` 同级的公开 `service/` 包，私有业务与 Table 保留在 `internal/`。HelloWorld 的独立入口使用 `service.Build`；组合示例通过 `service.NewHelloWorldService` 注入共享数据库，与其他服务挂载到同一个 Host。

`app.Run` 从 v0.5.0 开始提供；正式消费使用发布版本，本地联合开发可使用 `replace`。

## 通用 HTTP 行为（v0.3.1）

HTTPHost 默认安装请求 ID、请求日志、Recovery 和标准错误处理，并统一返回 404/405。日志记录方法、路由模板、状态与耗时；Recovery 在响应提交后不追加 JSON，适用于 SSE。

- `binder.Bind(c, req).JSON()` 与 `binder.BindJSON(c, req)`：只接受一个 JSON 文档，拒绝未知字段，限制请求体为 1 MiB，并运行 Gin 校验。非法参数返回 400，超限返回 413。
- `binder.QueryLimit(c, defaultLimit, maximum)`：解析正整数分页大小；默认值和上限由业务声明。
- `auth.ParseBearerTokens(raw)`：解析 token 到用户 ID 的 JSON 配置，要求非空、token 至少 24 字符、用户 ID 非空；`auth.BearerTokens(tokens)` 安装静态凭据鉴权，`auth.UserID(c)` 读取身份。
- `errors.Abort` / `errors.AbortWithError`：输出现有平铺错误契约；业务只负责领域错误映射。
- `stream.NewSSE(writer, writeTimeout)`：管理 SSE 响应头和每次写入期限；`SendWithID(id, event, data)` 发送带游标的事件，`Heartbeat()` 发送心跳。事件来源与回放规则由业务实现。
- `sdk.DecodeError(status, body)`：读取框架错误，保留 `Code`、`Message`、`RequestID` 和 `StatusCode`，`Status` 始终保存实际 HTTP 状态。非 JSON 失败仍返回可检查的 HTTP 错误。

Bearer 凭据、严格 JSON 绑定、分页解析、HTTP 中间件和 SDK 错误解码从 v0.3.1 开始提供；SSE 从 v0.3.0 开始提供。正式消费使用发布版本，本地联合开发可使用 Go workspace。

## 目录约定

| 目录 | 职责 |
| --- | --- |
| `cmd/server`、`cmd/cli` | 服务端与命令行入口 |
| `conf/server` | TOML 配置文件 |
| `service` | 可供独立入口和组合服务导入的服务构造、生命周期与业务装配 |
| `internal/app/<领域>` | 接口接入与业务逻辑 |
| `internal/table` | 数据模型 |
| `sdk/go`、`internalsdk` | 对外与内部客户端 |
| `doc/openapi` | Swagger 生成文件 |
| `deploy` | 部署文件与脚本 |

按实际需求创建目录。最小服务只需要入口和配置。完整组织规则见 [Go 项目组织约定](docs/go-project-layout.md)。代理协作规则见 [AGENTS.md](AGENTS.md)。配置占位符与加密说明见 [conf/README.md](conf/README.md)。

## 开发验证

安装 Go、C 编译器、Python 3、make 与 gorgeous 后，统一执行：

```sh
make check
```

gorgeous 的安装路径为 `github.com/fsgo/go_fmt/cmd/gorgeous`。检查覆盖根模块与四个独立示例的格式、测试和 vet，受影响并发路径的竞态检测、生成器回归，以及最小服务、流式服务和生成项目的实际启动、HTTP 请求与 SIGTERM 退出。生成项目在临时目录中检查，通过临时 `replace` 引用当前工作树；检查结束自动清理，不修改现有模块的依赖文件。

单独检查当前模块时仍可使用：

```sh
CGO_ENABLED=1 go test ./...
go vet ./...
```

`examples/example-hello-world`、`example-service1`、`example-service2` 和 `example-combine-service` 各自有 `go.mod`。修改跨模块依赖后，在每个示例目录执行 `go mod tidy`、`CGO_ENABLED=1 go test ./...`、`go vet ./...`。示例使用本地 `replace` 指向框架根目录。

## v0.5.0

新增 `app.Run` 和类型化配置装配，统一资源初始化、HTTP 运行与退出清理；SQL 日志使用框架 logger 并保留 Request ID。所有示例统一公开 `service/`、私有领域层和 context 约定，修正 SDK、CLI、Swagger 与打包流程。

最低 Go 版本升至 1.27。H2C 改用标准库 HTTP/2，客户端采用 prior knowledge 模式。Kubernetes 依赖与 kube-openapi 保持兼容版本。

## v0.4.0

升级 GORM PostgreSQL 驱动至 v1.6.0，使用 pgx/v5，移除旧 pgx/v4 驱动依赖；该版本最低 Go 版本为 1.25。

## v0.3.2

`db.Config.DSN` 支持完整 PostgreSQL URL 或连接字符串，保留调用方指定的 TLS、search_path、时区及其他驱动参数。未设置 DSN 时继续使用分字段配置。

## v0.3.1

HTTPHost 统一安装请求日志和 Recovery，返回标准 404/405；响应已提交后不追加错误 JSON。新增静态 Bearer 凭据与身份读取、严格单文档 JSON 绑定（1 MiB 上限）、分页大小解析及 SDK 标准错误解码。`HTTPBinder.JSON()` 现在拒绝未知字段和连续 JSON 文档，使用该接口的服务需确认请求符合约定。详见上方通用 HTTP 行为。

## v0.3.0

新增 `stream.NewSSE(writer, writeTimeout)` 与 `Send(event, data)`：逐事件编码和刷新，单次写入有期限，事件间等待不消耗写入时限。调用方负责操作 context、业务取消与错误事件。Console 代理支持带 charset 的 SSE，调试日志限制响应采集大小。完整用法见 [流式服务示例](examples/streaming-service/README.md)。

HelloWorld 模板的 SDK 方法新增 `context.Context` 参数，CLI 同步传递 context；HTTP 失败保留状态与服务错误，控制器在服务构造时创建并在路由重建时复用。升级已有模板 SDK 调用时补上传入的 context。新增 `make check` 统一执行框架、独立示例和生成项目验证。

## v0.2.0 接口变化

本版本修复 [设计审查](docs/server-go-design-review.md) 中的问题，并调整尚未稳定的公开接口。配置、数据库、幂等和订阅集合改由应用实例拥有，日志显式构造。`host.Service` 使用 `RegisterRoutes`、`Start(ctx)`、`Stop(ctx)`；`HTTPHost.Run(ctx)` 管理一次启动与退出，可选集成由 setup 安装。完整装配见最小示例和 HelloWorld 模板。

幂等路由必须先完成身份验证及本次授权，再用 `middleware.Idempotent` 包装最后的业务 handler。缓存按租户、用户、HTTP 操作和 token 隔离，只在单进程内有效；匿名请求不缓存。

SDK 构造函数返回错误。当前支持 API Key；AK/SK、STS 与内部 SDK 自动幂等尚未实现，配置这些能力会明确失败。内部请求包装继承入站请求的取消和截止时间。客户端克隆只在装配阶段使用，原客户端在所有借用者退出后关闭。

配置密文改用 AES-GCM，必须设置 `SERVER_SECRET_KEY`，旧 CBC 密文需要从原始明文重新加密。监听地址、协议、传输超时和基础设施配置变更需要重启进程；失败的动态更新保留上一份有效配置。

## 公开迁移

公开版本使用新的模块路径与清理后的首次提交。示例中的密码使用环境变量占位符，地址改为本地示例地址。

原私有日志依赖替换为本仓库 `logger` 包。许可证中间件接收 `middleware.LicenseAuthorizer`，调用方负责客户端生命周期。达梦数据库的私有驱动已移除；保留 MySQL、SQLite、PostgreSQL、ClickHouse、Kingbase 和 GaussDB 支持。
