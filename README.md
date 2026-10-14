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
go get github.com/FlyTOmeLight/server-go@v0.10.0
go mod tidy
go run ./cmd/server
```

修改配置中的 `AppName`、`AppCode` 和监听地址。业务入口放在 `cmd/<程序名>/`，公开服务构造与类型化装配放在同级 `service/`，私有业务逻辑放在 `internal/app/<领域>/`。框架默认从工作目录读取 `conf/server/config.toml`，也可设置 `SERVER_CONFIG_PATH`。

按需要选择可运行示例：

| 示例 | 适用场景 |
| --- | --- |
| [minimal-service](examples/minimal-service/README.md) | 只需要 HTTP 和配置 |
| [postgresql-service](examples/postgresql-service/README.md) | PostgreSQL CRUD、请求校验、默认值与 Go SDK |
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

生成器使用 HelloWorld 模板，并引用发布版框架 v0.10.0。按项目需求调整配置，API 文档可通过示例 Makefile 的 `doc` 目标重新生成。

## 当前工作树的依赖与 HTTP/2

Kubernetes 组件使用 v0.37.1；`kube-openapi` 固定为它们声明的 `v0.0.0-20260721132016-d427ff9ee9ad`，保持 schema v6 类型一致，不单独追逐该模块的最新提交。

`HTTPServer.UseH2C` 通过标准库 `http.Server.Protocols` 同时启用 HTTP/1 和明文 HTTP/2，退出由同一个 Server 排空请求并关闭连接。明文 HTTP/2 客户端使用 prior knowledge 模式，旧的 HTTP/1 Upgrade 升级方式不再使用。

## 应用启动（v0.5.0）

`app.Run(ctx, options, build)` 统一创建配置与日志、限制初始化时间、监听配置变化、运行 HTTPHost，并在退出或启动失败时清理资源。进程入口仍负责退出信号与退出码。

业务先声明配置结构体，字段名对应 TOML 节，名称不同时通过 `mapstructure` tag 指定 key；`app.Run` 从同一份配置快照一次解析全部字段，统一返回解析错误。装配函数接收初始化 context、`*app.Runtime` 和类型化配置指针，通过 `runtime.Initialize(ctx, modules...)` 初始化需要的资源，再通过 `runtime.AddService(svc)` 交付构造好的服务。资源按交付顺序逆序关闭；服务即使尚未启动，也会调用 `Stop`，成功后不重复调用，失败时允许重试。构造失败前尚未交付的部分资源仍由业务构造函数清理。

`app.Options` 可指定配置路径、日志配置、HTTP setup 和初始化/清理期限，装配与服务启动共享默认 15 秒预算，每次应用清理操作默认先等待 5 秒。HTTP 请求排空和运行中服务停止的首次等待预算仍由 HTTPHost 管理。停止超时后继续等待 worker 退出，再关闭依赖；其他停止错误未恢复时保留依赖并返回错误。初始化 context 不用于启动长期任务；任务通过 `host.Service.Start(ctx)` 获取运行 context，启动期间响应进程取消，启动完成后保留到 HTTP 排空结束。

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

数据库事务使用 [`db.Transaction`](db/README.md)，支持显式隔离级别、只读事务、context 取消及有次数和时间预算的整笔事务重试。

数据库 SQL 日志统一通过框架 logger 输出：普通查询为 Info，超过 200 毫秒的慢查询为 Warn，失败查询为 Error，并遵守日志实例的级别过滤。RequestID 中间件将请求 ID 写入标准请求 context，GORM 日志适配器从每次操作的 context 读取日志实例和 Request ID。初始化与迁移使用应用日志实例。

HTTP Controller 和对应业务 Service 使用 `*core.Context`；数据库和 SDK 操作传递 `ctx.Request.Context()`，保留取消、超时和 Request ID，避免数据库后台任务读取被 Gin 复用的对象。

可组合的服务放在与 `cmd` 同级的公开 `service/` 包，私有业务与 Table 保留在 `internal/`。HelloWorld 的独立入口使用 `service.Build`；组合示例通过 `service.NewHelloWorldService` 注入共享数据库，与其他服务挂载到同一个 Host。

`app.Run` 从 v0.5.0 开始提供；正式消费使用发布版本，本地联合开发可使用 `replace`。

## v0.6.0 接口调整

本次设计修复记录在 [issues #1–#9](https://github.com/FlyTOmeLight/server-go/issues)，已发布为 [v0.6.0](https://github.com/FlyTOmeLight/server-go/releases/tag/v0.6.0)：

- `HTTPHost.Run(ctx)` 默认限制服务启动为 15 秒；`RunWithStartupTimeout(ctx, timeout)` 可指定启动预算。`Start` 必须响应取消；`Stop` 返回 nil 才表示全部任务已退出，失败后允许再次等待或清理。
- `NewLeaderElection`、`NewK8sLeaderElection` 返回实例和错误；`Run(ctx)` 返回错误，`RunAsync(ctx)` 返回完成结果 channel。Kubernetes 配置错误直接失败，本地单节点运行显式使用 `NewDummyAppElection` 或示例的 `--local`。
- 删除 `AuthWithAuthCenter`。可信网关身份使用 `auth.TrustedIdentityHeaders(verify)`，验证网关凭据、来源和身份 Header 的控制权后才采纳身份；通过 setup 闭包调用 `httpintegration.InstallWithAuth(engine, cfg, authenticate)`。仅配置 `AuthCenter` 而未提供验证 handler 会启动失败。`auth.Use` 现在返回错误。
- `auth.Impersonate(authorize)` 返回 handler 和错误，必须在身份认证后安装；授权函数检查调用方是服务账号并允许代理指定组织与用户。代理 Header 不再由默认集成隐式处理。
- 原生 SQL 使用目标数据库语法；删除连接池自动改写与 `ConvertSQLIdentifiers`。整数 `tinyint` 映射 `SMALLINT`，布尔字段按实际方言模式映射和绑定。
- 配置加载时一次解析全部占位符，快照固定有效值。外部来源变化后执行 `Reload`；文件监听仍只监听主配置文件。程序化占位符输入使用 `conf.FromSettings`，`NewManagerFromConfig` 接收已解析值。
- 分页 query 参数与 JSON 字段一致，负页码和页大小返回 400，零值使用默认分页。框架错误支持标准 `%w` 包装；cron 多个同步来源合并后统一更新任务。

## 通用 HTTP 行为（v0.3.1）

HTTPHost 默认安装请求 ID、请求日志、Recovery 和标准错误处理，并统一返回 404/405。日志记录方法、路由模板、状态与耗时；Recovery 在响应提交后不追加 JSON，适用于 SSE。

- `binder.Bind(c, req).JSON()` 与 `binder.BindJSON(c, req)`：只接受一个 JSON 文档，拒绝未知字段，限制请求体为 1 MiB，并运行 Gin 校验。非法参数返回 400，超限返回 413。
- `binder.QueryLimit(c, defaultLimit, maximum)`：解析正整数分页大小；默认值和上限由业务声明。
- `auth.ParseBearerTokens(raw)`：解析 token 到用户 ID 的 JSON 配置，要求非空、token 至少 24 字符、用户 ID 非空；`auth.BearerTokens(tokens)` 安装静态凭据鉴权，`auth.UserID(c)` 读取身份。
- `errors.Abort` / `errors.AbortWithError`：输出现有平铺错误契约；业务只负责领域错误映射。
- `stream.NewSSE(writer, writeTimeout)`：管理 SSE 响应头和每次写入期限；`SendWithID(id, event, data)` 发送带游标的事件，`Heartbeat()` 发送心跳。事件来源与回放规则由业务实现。
- `sdk.DecodeError(status, body)`：读取框架错误，保留 `Code`、`Message`、`RequestID` 和 `StatusCode`，`Status` 始终保存实际 HTTP 状态。非 JSON 失败仍返回可检查的 HTTP 错误。

Bearer 凭据、严格 JSON 绑定、分页解析、HTTP 中间件和 SDK 错误解码从 v0.3.1 开始提供；SSE 从 v0.3.0 开始提供。正式消费使用发布版本，本地联合开发可使用 Go workspace。

## 请求约定（v0.7.0）

SDK 的请求结构体用 `binding` tag 声明必填、长度、范围和枚举，用 `json`、`form`、`uri` 指定参数来源。`binder.All()` 合并正文、query 和 URI 后统一校验，路径字段使用 `json:"-"`。优先复用已有 validator；跨业务通用的校验可加入 `pkg/binder`，数据库存在性、权限等领域规则留在 Service。

Controller 对声明了静态 `default` tag 的请求，在绑定前调用 `defaults.Set(&req)` 并处理错误。客户端显式传入的零值会覆盖默认值；局部更新要保留未传字段，不能直接补默认值。完整示例见 [PostgreSQL 服务](examples/postgresql-service/README.md) 和 [请求约定](docs/go-project-layout.md)。这批请求处理改动从 v0.7.0 提供；使用新增 URI 必填标签时，框架消费版本至少为 v0.7.0。

## 发布约定

影响调用方的修复和功能合入 main 后发布版本；纯文档和测试变更可合并到下一次发布。兼容修复使用 patch，新增能力使用 minor；v0.x 阶段的破坏性接口调整也升级 minor，并说明迁移方法。标签指向通过完整检查的 main 提交，同步示例与生成器的消费版本。正式项目引用发布版本，本地联合开发使用 `replace`。

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

安装 Go、C 编译器、Python 3、make、gorgeous 和 Docker，并启动 Docker daemon 后，每次框架修改都执行：

```sh
make check
```

格式工具使用 [FlyTOmeLight/go_fmt](https://github.com/FlyTOmeLight/go_fmt) v0.9.0，CI 固定到该版本的提交 `9d389e31b15509ae9cbdff326499929f715db78f`。在 server-go 仓库外安装：

```sh
git clone --depth 1 --branch v0.9.0 https://github.com/FlyTOmeLight/go_fmt.git
cd go_fmt
go install ./cmd/gorgeous
```

该仓库保留原模块路径，因此通过仓库源码安装。将 `GOBIN`（未设置时为 `$(go env GOPATH)/bin`）加入 `PATH`。

检查覆盖根模块与四个独立示例的格式、测试和 vet、并发路径的竞态检测、所有可运行示例及生成项目的实际启动、HTTP/SDK/CLI 请求与 SIGTERM 退出。数据库检查创建独立 PostgreSQL 17 和 MySQL 8.4 临时容器，验证真实双库事务与死锁重试。PostgreSQL 示例检查验证真实数据库 CRUD、字段类型、默认值和参数错误；缺少 Docker 时检查失败。生成项目通过临时 `replace` 引用当前工作树。检查结束清理进程、容器和临时文件，不修改现有模块的依赖文件。GitHub Actions 在 push 和 PR 时执行同一条 `make check`。

单独检查当前模块时仍可使用：

```sh
CGO_ENABLED=1 go test ./...
go vet ./...
```

`examples/example-hello-world`、`example-service1`、`example-service2` 和 `example-combine-service` 各自有 `go.mod`。修改跨模块依赖后，在每个示例目录执行 `go mod tidy`、`CGO_ENABLED=1 go test ./...`、`go vet ./...`。示例使用本地 `replace` 指向框架根目录。

## v0.10.0

新增 `db.Transaction`，提供显式 SQL 隔离级别与只读选项、context 取消、带次数和时间预算的 PostgreSQL/MySQL 整笔事务重试。嵌套调用拒绝执行，现有 GORM 事务接口保持不变。使用方式及回调边界见 [事务文档](db/README.md)。完整检查新增 MySQL 8.4 隔离数据库与双库真实死锁测试。

## v0.9.0

所有示例按当前推荐统一 Controller 错误处理、SDK 构造和资源交付。18 个业务 HTTP 接口维护完整 Swagger 注释；每个 HTTP 示例提供 `make doc`，生成文档与参数来源、默认值、可空字段、状态码和 SSE 事件一致。生成器沿用更新后的 HelloWorld 模板。

### 示例 SDK 迁移

HelloWorld、Dataset 和 HelloWorld 内部 SDK 的便捷构造从 `NewClient(ak, sk, endpoint)` 改为 `NewClient(endpoint)`。API Key、代理和内部身份传播使用各自的 `NewClientWithConfig`，客户端由调用方关闭。框架根 SDK 的公开构造接口保持原契约。SSE 的非 JSON HTTP 失败统一返回 `HTTPError`，并保留实际 HTTP 状态。

普通 Controller 调用 `serrors.AbortWithError(ctx, err)` 完成错误响应后返回，业务 Service 只返回错误。新业务 API 默认采用资源路由；已有 HelloWorld 的全 POST 接口保持原路径和方法。详细约定见 [Go 项目组织约定](docs/go-project-layout.md)。

## v0.8.0

JSON 的 1 MiB 读取预算覆盖 Debug、幂等与 binder 的组合，前置中间件不再先读完整个超限正文。缺少 Content-Type 的调试请求使用相同预算；显式文本、表单和 multipart 保留原有规则。多个读取者复用本次请求的正文。

立即错误和延后错误共享完成规则，保留请求 ID、业务错误码和已开始输出后的行为。自动转为 HTTP 200 的错误仍被识别为失败，不进入幂等缓存。应用与 HTTPHost 共享任务停止策略，超时后继续等待并保留错误，停止失败时保留依赖。

普通 JSON SDK 方法用 `sdk.Complete(result, err)` 统一完成请求，保留网络、取消与解码错误身份以及 HTTP 错误元数据。SSE 继续使用独立的流读取和关闭流程。

`CommandLine` 实例拥有配置、Apply 注册、SDK 和日志。业务命令改用 `cli.Client(cmd.Context())` 获取本次执行的客户端，通过 `cli.RegisterCommandApplyHandler` 注册领域 Apply；`ExecuteContext` 返回错误并清理资源，`Execute` 处理进程信号和退出码。`CommandLine` 不再初始化包级 `cli.Config`，旧手动 `SetConfig` 路径仍可使用。迁移示例见 [Go 项目组织约定](docs/go-project-layout.md)。这些变更从 v0.8.0 提供。

生成器按明确的模板文件与完整片段展开业务领域，不再根据每行是否包含 `Module` 猜测复制范围。新增模板领域文件或改变聚合位置时，同步更新 `scripts/gen_code.py` 的规则；规则失配会停止生成，已有目标继续受保护。

## v0.7.0

`binder.All` 合并正文、query 和 URI 后统一校验，支持 SDK 请求的必填路径标签。Controller 在绑定前设置静态默认值；Dataset 创建省略 fileFormat 时使用 File，显式空串或未知枚举返回 400。CLI apply 单独映射路径字段，Swagger 同步请求契约。

新增 PostgreSQL CRUD、分页与 Go SDK 示例，验证真实数据库类型、NULL、默认值、取消和资源关闭。每次修改由 `make check` 和 GitHub Actions 实际运行全部示例及生成项目。

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
