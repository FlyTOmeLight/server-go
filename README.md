# server-go

基于 Gin 的 Go 服务框架，提供统一应用生命周期、配置与占位符热更新、鉴权与身份传播、可观测性、HTTP 安全防护与幂等、多数据库/MongoDB/Redis/Dapr 集成、Go SDK 及 CLI 工具集。

- **模块路径**：`github.com/FlyTOmeLight/server-go`
- **环境要求**：Go `1.27+`；使用 SQLite 的示例或测试需要启用 `CGO_ENABLED=1` 并安装 C 编译器；不启用 SQLite 的服务可直接以 `CGO_ENABLED=0` 构建运行
- **依赖与协议**：全部依赖均为公开模块，采用 [Apache-2.0](LICENSE) 协议开源

## 快速开始

### 运行最小示例

最小示例不依赖数据库、Redis 或外部鉴权服务：

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

按 `Ctrl+C` 停止服务。框架默认提供 `/ping`、`/healthz`、`/livez`、`/readyz` 与 `/version` 端点。示例配置使用 `Fake` 鉴权，仅适用于 `debug` 或 `test` 模式；`RunMode = "release"` 下未提供显式鉴权中间件将拒绝启动。

### 创建新业务服务

从 `examples/minimal-service` 复制 `cmd/`、`service/`、`internal/` 和 `conf/`，把示例代码中的 import 替换为自己的模块路径，然后初始化依赖并启动：

```sh
go mod init github.com/your-org/your-service
go get github.com/FlyTOmeLight/server-go@v0.17.1
go mod tidy
go run ./cmd/server
```

修改配置中的 `AppName`、`AppCode` 与监听地址；配置默认读取工作目录下的 `conf/server/config.toml`，也可通过 `SERVER_CONFIG_PATH` 指定。完整的生产装配模板、中间件推荐顺序及全栈能力复用方法见 [业务项目接入与能力复用指南](docs/integration-guide.md)。

### 使用代码生成器

从仓库根目录生成标准单模块或多模块 CRUD 服务（含服务端、Go SDK、Swagger 与 CLI）：

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

生成器使用 HelloWorld 模板，并引用发布版框架 v0.17.1。按项目需求调整配置，API 文档可通过示例 Makefile 的 `doc` 目标重新生成。

## 可运行示例

| 示例 | 独立 `go.mod` | 默认监听 | 存储 / CGO | 核心能力演示 |
| --- | --- | --- | --- | --- |
| [minimal-service](examples/minimal-service/README.md) | 否（根模块） | `127.0.0.1:8080` | 无 / `CGO_ENABLED=0` | `app.Run`、最小配置与优雅退出 |
| [postgresql-service](examples/postgresql-service/README.md) | 否（根模块） | `127.0.0.1:18080` | PostgreSQL / `CGO_ENABLED=0` | `RunMode="release"`、CRUD、`defaults.Set`、`binder.All`、Go SDK |
| [example-hello-world](examples/example-hello-world/README.md) | 是 | `127.0.0.1:8080` | SQLite / 服务端 `CGO_ENABLED=1` | 生成器模板、RPC POST CRUD、Go SDK、Cobra CLI、`apply` |
| [example-service1](examples/example-service1/README.md) | 是 | `127.0.0.1:8080` | SQLite / 服务端 `CGO_ENABLED=1` | RESTful 数据集 CRUD、软删除、`sdk.Complete` 与 CLI |
| [example-service2](examples/example-service2/README.md) | 是 | `127.0.0.1:8080` | 无 / `CGO_ENABLED=0` | 无状态 HTTP 服务与公开 `service` 构造函数 |
| [example-combine-service](examples/example-combine-service/README.md) | 是 | `127.0.0.1:8080` | 共享 SQLite / 服务端 `CGO_ENABLED=1` | 单 `HTTPHost` 组合三个服务与组合 CLI |
| [streaming-service](examples/streaming-service/README.md) | 否（根模块） | `127.0.0.1:8080` | 无 / `CGO_ENABLED=0` | `stream.NewSSE`、取消传播、流式 SDK |
| [example-leader-election](examples/example-leader-election/README.md) | 否（根模块） | 无 HTTP | 无 / `CGO_ENABLED=0` | 独立 worker、K8s Lease 选主与 `--local` 单节点模式 |

更多运行命令、打包规范与接口约定见 [示例索引](examples/README.md) 与 [Go 项目组织约定](docs/go-project-layout.md)。

## 包总览

| 包 | 职责与核心能力 |
| --- | --- |
| 根包 (`server`) | `Module` 接口、`NewBootstrap`（失败回滚、逆序关闭）、HTTP/Dapr 响应助手（`OK`、`Abort`） |
| `app` | 泛型入口 `app.Run[T]`、`Options`、`Runtime`（单次解析类型化配置、初始化资源、配置监听、逆序关闭） |
| `conf` | `Manager`、`Load`/`FromSettings`、`${env:}`/`${file:}`/`${secret:}` 占位符解析、AES-GCM 加解密与防抖监听 `Watch`（支持 K8s ConfigMap `..data`） |
| `host` | `HTTPHost`（路由构建、热更新/重启边界、健康探针、`AdminListen`、TLS/mTLS、`UseH2C`、优雅停机）与 `ModbusHost` |
| `httpintegration` | `Install`、`InstallWithAuth`、`InstallDiagnostics`（统一装配调试日志、诊断端点、`/feature`、静态资源、Console 代理与鉴权） |
| `auth` | `Identity`（同步至 `context.Context`）、`Use`、`FakeAuth`、`BearerTokens`、`TrustedIdentityHeaders`、`InternalPropagatedIdentity`、`Impersonate` |
| `middleware` | 请求 ID、请求日志、Recovery、健康检查、`Tracing`、`CORS`、`SecurityHeaders`、`RateLimit`/`MaxInFlight`、`RequestTimeout`、`MaxBodyBytes`、`Idempotent`、`HTTPAuthorization`、`LicenseAuth`、`Asset`、`Debug`、`Feature` |
| `idempotence` | `Store` 接口、本地有界 `Controller`（`New`/`NewWithOptions`）与基于 Redis Lua 脚本的多副本 `RedisStore` |
| `errors` | 统一 `Error` 契约、`Abort`/`AbortWithError`、`ErrorHandler`（含 API 错误码转换）、标准库 `%w` 兼容的 `Wrap`/`WithMessage` |
| `logger` | 基于 Zap 与 Lumberjack 的分级分流日志、`WithContext`/`FromContext`（自动提取 `requestId`/`traceId`/`spanId`）及 `log/slog` 桥接（`With`/`SlogHandler`/`Slog`） |
| `metrics` | 默认 Prometheus 端点 `Metrics()`，以及独立注册表 `NewRegistry`、`HandlerFor` 与低基数 HTTP 路由指标 `NewHTTPMetrics` |
| `stream` | `NewSSE`：管理 SSE 响应头、`Send`/`SendWithID`/`Heartbeat` 逐事件刷新与分段写入超时控制 |
| `console` | FE 与 OpenAPI 适配器反向代理 `RegisterConsoleAPI`（`GET`/`POST`/`PUT`/`PATCH`/`DELETE`、响应包装、gzip、SSE、WebSocket 重路由） |
| `proxy` | 通用 HTTP 反向代理 `NewReverseProxy`（目标 URL 校验、`X-Forwarded-*`、连接池复用） |
| `swagger` | `Swagger()` 与按引擎隔离的 `SwaggerWithPrefix(prefix)`，配合 `conf.HTTPServerConfig.SwaggerDocPrefix` 支持多服务实例 |
| `version` | 构建信息注入变量、`Version()` 处理器与标准库 `GetVersionByURL`/`GetVersionByURLContext` |
| `db` / `db/migrate` / `db/mongodb` | 六种 SQL 驱动与池校验、参数脱敏 GORM 日志、`db.Transaction`、`UTCTime`/`Time`；`db/migrate` 版本化迁移；解耦的 `db/mongodb` 客户端、`Time` 与 `NewError` |
| `redis` / `pubsub` / `cron` | `NewRedis`/`NewRedisLazy` 与 `redis.Module`；Dapr 订阅 `Registry` 与可复用连接的 `Publisher`；`TaskManager`/`PeriodTask` 调度器 |
| `sdk` / `internalsdk` / `cli` | 基于 Resty v3 的 HTTP 客户端（`Request-ID` 与追踪传播、幂等重试、`Complete`、`DecodeError`、`PagingRequest`）、内部身份转发及 Cobra `CommandLine` |
| `pkg/*` / `naming` / `constants` | `core.*` Gin 类型别名、`NewID`/`NewUUID`、`ParseFile`、`StructDecode`、`pkg/binder`、`pkg/cache`、`pkg/election`、`pkg/modbus`、`pkg/safety`、`pkg/datatypes`、`pkg/compare`、`pkg/utils`、`pkg/values`、`naming`、`constants` |

## 应用启动与配置

`app.Run(ctx, options, build)` 负责创建应用日志、加载配置并启动热更新监听、解析业务配置结构体、装配并运行 `HTTPHost`，并在退出或启动失败时按交付逆序释放资源（进程入口仍负责信号监听与退出码）。

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

- **预算与清理语义**：`Options.InitTimeout` 默认 `15s`（共享于 `build` 与服务 `Start`），`Options.CloseTimeout` 默认 `5s`（单步清理超时）。退出时先通过 `RunWithShutdownHook` 关闭配置监听并取消 `runCtx`，再排空 HTTP 请求并逆序停止服务与模块；若服务停止超时会继续等待 worker 退出后再关闭依赖，若服务返回其他未恢复停止错误则保留模块依赖以便重试，并在最后执行 `log.Close()`。
- **配置加载与占位符**：`conf.Manager` 支持 `${secret: v1:<base64>}`（AES-GCM，密钥由环境变量 `SERVER_SECRET_KEY` 指定）、`${env: NAME[:default]}` 与 `${file: /path}` 占位符，详见 [conf/README.md](conf/README.md)。
- **动态热更新边界**：
  - **支持热更新**（原子重建并切换路由器）：`RunMode`、`Feature`、`FakeAuth`，以及 `HTTPServer` 中除监听与传输外的字段（`APIBasePath`、`PprofEnable`、`MetricsEnable`、`SwaggerEnable`、`SwaggerDocPrefix`、`Trace`、`AuthType`、`ConsoleEnable`、`ConsoleProxyNames`、`AltoHandlerAPIError`、`Assets`、`AllowAPIPathNoHandlerAPIError`）。
  - **需重启进程**（准备阶段返回 `host.ErrRestartRequired` 并保留原配置）：`AppName`、`AppCode`、`DeployMode`、`Services`，以及 `Listen`、`AdminListen`、`ReadTimeout`、`ReadHeaderTimeout`、`WriteTimeout`、`IdleTimeout`、`PreStopDelay`、`MaxHeaderBytes`、`UseH2C`、`TLS`、`TrustedProxies`。

## HTTP 生命周期、探针与安全中间件

- **内置端点与优雅下线**：路由器默认启用请求 ID、请求日志、Recovery、统一 `ErrorHandler` 与标准 404/405，并安装 `/ping`、`/healthz`、`/livez`、`/readyz`（通过 `HTTPHost.RegisterHealthCheck` 注册自定义就绪检查，单轮 2 秒超时）及主路由 `/version`。停机时先将 `/readyz` 标为不可用，等待 `PreStopDelay` 毫秒后再排空连接（5 秒排空预算）并逆序停止业务服务。
- **端口隔离与传输安全**：默认超时为 `ReadTimeout` 20s、`ReadHeaderTimeout` 5s、`WriteTimeout` 10s、`IdleTimeout` 10s、`MaxHeaderBytes` 100000。设置 `HTTPServer.AdminListen` 后，`pprof`、`metrics` 与 `Swagger` 自动迁移到独立的明文 HTTP 管理端口，不再暴露于业务主端口（可通过 `HTTPHost.AdminAddress()` 获取实际地址）；主端口支持 `HTTPServer.TLS`（TLS 1.2+ 及可选 `ClientCAFile` mTLS 客户端证书校验）、`TrustedProxies` 及基于标准库 HTTP/2 的 `UseH2C`（prior knowledge 模式）。
- **请求绑定、默认值与错误契约**：静态默认值在绑定前调用 `defaults.Set(&req)`（保留局部更新的显式零值），参数绑定使用 `binder.Bind(c, &req).All()` 或 `.JSON()`（严格单 JSON 文档、拒绝未知字段、1 MiB 限制），错误使用 `errors.Abort` / `errors.AbortWithError` 输出平铺 JSON 响应。Controller 与业务 `core` Service 传递 `*core.Context`，数据库、SDK 与后台操作必须使用 `c.Request.Context()`。
- **防护、幂等与代理扩展**：
  - 防护中间件：`CORS`、`SecurityHeaders`、`RateLimit`（按 key 有界 LRU 令牌桶）、`MaxInFlight`、`RequestTimeout`、`MaxBodyBytes`。
  - 幂等控制：作为鉴权与授权完成后的最后一个 handler 挂载（仅对 `POST` + `application/json` + 已认证请求 + `Idempotency-Key`/`clientToken` 生效）；本地使用有界 `idempotence.Controller`（默认 24h TTL / 10000 条 / 1 MiB 响应体），多副本使用 `idempotence.NewRedisStore`（Lua 原子锁与结果写回，默认 24h TTL / 1m `LockTTL` / 50ms 轮询 / 1 MiB）。
  - 扩展能力：`stream.NewSSE` 分段写超时控制、`proxy.NewReverseProxy`、以及 `console.RegisterConsoleAPI` / `middleware.Asset` / `middleware.Debug` / `middleware.LicenseAuth`。

## 鉴权与身份传播

框架在 `RunMode = "release"` 下默认 fail-closed（未传入显式鉴权 handler 时拒绝 `FakeAuth` 或空 `AuthType`）。所有内置鉴权中间件及外部适配均将 `auth.Identity` 同步写入标准请求 `c.Request.Context()`（通过 `auth.IdentityFromContext(ctx)` 读取），入口统一收敛到 `constants.KeyUserID`、`KeyTenantID`、`KeyProjectID`：

1. **可信网关入口**：`auth.TrustedIdentityHeaders(verify)` 验证网关来源后读取 `x-forge-user-id`、`x-forge-tenant-id` 与可选的 `x-forge-project-id`（拒绝同名多值头与重复鉴权覆盖，不写 `KeyImpersonateOrigin*`）。
2. **凭据与外部中间件**：`auth.BearerTokens(tokens)` 使用预计算 SHA-256 摘要做常量时间比对（`auth.ParseBearerTokens` 强制 token ≥ 24 字符）；标准 `net/http` 鉴权中间件可通过 `middleware.HTTPAuthorization` 挂载到 Gin。
3. **内部透传与代理**：服务间调用使用 `internalsdk.Wrap(client, c)` 自动转发 `x-impersonate-target-{user,tenant,project}-id`；下游先经 `auth.BearerTokens` 验证调用方身份，再通过 `auth.InternalPropagatedIdentity(authorize)` 同时写入 `Key*` 与 `KeyImpersonateOrigin*`（`AuthMode = "Internal"`）。真实的跨用户代理使用 `auth.Impersonate(authorize)`（`AuthMode = "Impersonate"`，origin 记录调用方本身）。

## 可观测性

- **结构化日志（`logger`）**：复用 Zap 与 Lumberjack（请求 ID/级别过滤/轮转），`FromContext` 自动附加 `requestId`、`traceId`、`spanId`；通过 `With(...)`、`SlogHandler()` 与 `Slog()` 无缝桥接标准库 `log/slog`。`middleware.RequestLogWithOptions` 支持 `SkipPaths`、可选身份字段与按状态码分流日志级别（原 `RequestLog()` 保持兼容）。
- **Prometheus 指标（`metrics`）**：默认 `/metrics` 使用 `metrics.Metrics()`；需要按服务隔离注册表时使用 `metrics.NewRegistry()`、`metrics.HandlerFor(gatherer)` 与 `metrics.NewHTTPMetrics(reg, namespace)`（基于匹配路由模板统计低基数 `http_requests_total`、`http_request_duration_seconds`、`http_requests_in_flight`）。
- **分布式追踪（`middleware.Tracing` & `sdk`）**：`middleware.Tracing` 接入 OpenTelemetry 服务端 span 与 `traceparent` 上下文提取，支持 `WithTracerProvider`、`WithPropagator`、`WithSkipPaths`；`sdk.Client` 自动向出站请求注入 `Request-ID` 与追踪上下文。

## 数据存储、消息与后台任务

- **关系型数据库（`db` & `db/migrate`）**：支持 `MySQL`、`SQLite`、`PostgreSQL`、`ClickHouse`、`Kingbase`、`GaussDB`。默认通过参数脱敏 GORM 日志避免日志打印明文参数，并对慢查询（>200ms）告警；`db.Transaction` 提供显式隔离级别、只读事务与 PG/GaussDB（`40001`/`40P01`）/ MySQL（`1213`）整笔重试（详见 [db/README.md](db/README.md)）；`db/migrate` 提供 `LoadFS`、`New`、`NewFromGORM` 与校验和保护的 `Up`/`Down`/`Status`。
- **MongoDB 与 Redis（`db/mongodb` & `redis`）**：`db/mongodb` 与根 `db` 包完全解耦，独立提供 `Module`、`NewClientWithConfig`、`Time`（BSON 纳秒序列化）及 `NewError`；`redis` 提供 `NewRedis`、`NewRedisLazy`、`Module{Lazy: true}`、`XReadArgs` 与常量 `redis.Nil`。
- **消息、调度与选主（`pubsub`、`cron`、`pkg/election`）**：`pubsub` 提供线程安全订阅注册表 `Registry` 与可复用 Dapr gRPC 连接的 `Publisher`（`NewPublisher` / `NewPublisherWithPort` / `Close`）；`cron` 提供 `TaskManager`/`PeriodTask`、时区校验、统一 `SetErrorHandler` 与 worker panic 恢复；`pkg/election` 提供 Kubernetes `LeaseLock` 选主（默认 `15s/10s/2s` 与失主指数退避）及本地单节点 `NewDummyAppElection`。

## SDK 与 CLI

- **`sdk` / `internalsdk`**：`sdk.NewClientWithConfig` 支持 API Key 鉴权与 `ProxyURL` 校验，自动从 `context.Context` 透传 `Request-ID` 与 OTel 追踪上下文；仅对幂等方法（`GET`/`HEAD`/`OPTIONS`/`PUT`/`DELETE`）或显式携带 `Idempotency-Key`/`clientToken` 的请求启用失败重试；提供 `sdk.Complete`、`sdk.DecodeError` 与防注入白名单分页排序 `PagingRequest.DBOrderWithAllowlist`。`internalsdk.Wrap` 额外支持服务间身份代理头透传。
- **`cli`**：基于 Cobra 与 Viper 的命令行框架，提供 `CommandLine`（`ExecuteContext` 自动绑定上下文与释放资源）、`cli.Client(cmd.Context())`、`RegisterCommandApplyHandler`、`apply -f/-d`、`config get/set/unset`（`0600` 文件权限与敏感配置脱敏）、`encrypt --stdin`/管道加密、`version` 与多格式输出。

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

按实际需求创建目录。业务项目从零接入与跨模块能力复用见 [业务项目接入与能力复用指南](docs/integration-guide.md)，目录职责与 SDK 契约见 [Go 项目组织约定](docs/go-project-layout.md)，配置占位符见 [conf/README.md](conf/README.md)，代理协作规则见 [AGENTS.md](AGENTS.md)。

## 开发验证与发布

### 格式化工具安装

格式检查固定使用 [FlyTOmeLight/go_fmt](https://github.com/FlyTOmeLight/go_fmt) `v0.9.0`（提交 `9d389e31b15509ae9cbdff326499929f715db78f`，与 GitHub Actions CI 一致）。在仓库外安装并确保 `GOBIN`（或 `$(go env GOPATH)/bin`）在 `PATH` 中：

```sh
git clone --depth 1 --branch v0.9.0 https://github.com/FlyTOmeLight/go_fmt.git
cd go_fmt
go install ./cmd/gorgeous
```

### 运行测试与全仓检查

```sh
make check       # 全量校验：版本一致性 + 5 模块测试/格式/vet + -race + Python 单测 + Docker PG 17/MySQL 8.4 + 8 示例及生成项目实测
make check-fast  # 本地快速校验：版本一致性 + 全模块测试/格式/vet + -race + Python 单测（跳过 Docker 与示例启停）
```

也可在根目录或四个独立示例目录（`examples/example-hello-world`、`example-service1`、`example-service2`、`example-combine-service`）中单独运行 `CGO_ENABLED=1 go test ./...` 与 `go vet ./...`。

### 依赖锁定与发布规范

- **依赖基线**：Go `1.27.0`；Kubernetes 组件固定使用 `v0.37.1` 与配套 `kube-openapi`（`v0.0.0-20260721132016-d427ff9ee9ad`），避免 schema v6 类型不一致。
- **发布规则**：影响调用方的修复或功能合入 `main` 后打版本标签（兼容修复升 patch，新增能力或 `v0.x` 不兼容调整升 minor 并附迁移说明）；不重写既有标签，每次发版前必须同步检查并更新 [README.md](README.md) 与 [docs/integration-guide.md](docs/integration-guide.md)（能力清单、接入示例、配置边界与发布说明），同时更新示例依赖与 `scripts/gen_code.py`（版本号一致由 `scripts/check.py` 自动校验）。

## v0.17.1

- 文档重写与接入指南：结合全仓源码重写 [README.md](README.md)，按当前能力体系梳理快速开始、8 个示例、全包总览、应用生命周期、HTTP/鉴权/可观测性/数据/SDK/CLI 契约与版本演进摘要。
- 新增 [业务项目接入与能力复用指南](docs/integration-guide.md)：提供全框架能力速查表（避免业务项目重复造轮子）、新建与存量 Gin 服务迁移路径、生产 `config.toml` 模板与热更新边界、中间件分层装配顺序、全模块接入示范及 Kubernetes 部署自查清单。
- 发布规范：在 [AGENTS.md](AGENTS.md) 与 [README.md](README.md) 明确每次发版前必须同步检查并更新 `README.md` 与 `docs/integration-guide.md`。

## v0.17.0

- **依赖瘦身与基础包加固（含迁移提示）**：移除 `github.com/go-resty/resty/v2`、`github.com/pkg/errors`、`github.com/mitchellh/mapstructure`、`github.com/gookit/goutil`、`github.com/patrickmn/go-cache` 5 个冗余或停止维护的直接依赖。`version.GetVersionByURL` 改用标准库 `net/http` 并新增 `GetVersionByURLContext`；`errors.Wrap`/`WithMessage` 改用标准库 `%w` 保留错误链；`pkg/struct_formatter` 迁移至 `mapstructure/v2` 并修复 `FormToStringMap` 并发写入调用方指针的竞态；`pkg/cache` 改用标准库懒过期实现，不再泄漏后台清理协程并新增 `Close()`；`pkg/safety` 修复 `PKCS7Padding` 底层切片覆写、`UnPKCS7Padding` 改为恒定时间校验与统一错误并预编译 `IsBase64` 正则。根 `db` 包不再依赖 `go.mongodb.org/mongo-driver` v1（**迁移提示**：MongoDB BSON 时间序列化改用 `mongodb.Time`，MongoDB 错误映射改用新增的 `mongodb.NewError`；`db.Time` 保留 SQL `Value`/`Scan` 兼容）。
- **Console 代理、Pubsub 与多服务 Swagger**：`console` 加固反向代理（支持 `GET`/`POST`/`PUT`/`PATCH`/`DELETE`、按路径段重写 `/api`、封装空响应体 4xx/5xx 避免误报 502、支持 `gzip` 解压与 SSE 实时刷新/清写超时、WebSocket 重路由防 `RawPath` 损坏、移除 `%#v` 防止日志泄露请求头）；`pubsub` 新增可复用连接的 `Publisher`（`NewPublisher`/`NewPublisherWithPort`/`Close`），修复 `OnEventWithGroup` 对空回调返回值与斜杠拼接的处理；`swagger` 新增 `SwaggerWithPrefix` 并在 `conf.HTTPServerConfig` 新增 `SwaggerDocPrefix`，支持多引擎独立 Swagger 文档实例。
- **示例、代码生成器与检查流水线**：修复 `example-hello-world` CLI `delete` 子命令示例错误；示例 `Makefile` 统一支持 `SWAG` 覆盖/`PATH`/`GOBIN`/`GOPATH/bin`，非 SQLite 示例构建与运行改为 `CGO_ENABLED=0`；`scripts/gen_code.py` 生成的 `go.mod` 对齐模板直接依赖并规范制表符缩进；`scripts/check.py` 新增跨文件发布版本一致性校验 `check_release_versions` 与 `--fast` 无 Docker 快速检查模式（对应根 `Makefile` `make check-fast`），并将 `./pkg/cache`、`./pkg/safety`、`./version`、`./swagger` 纳入 `-race` 覆盖。

### 历史版本摘要

完整发布说明见 [GitHub Releases](https://github.com/FlyTOmeLight/server-go/releases)；涉及需要调用方注意的迁移节点如下：

| 版本 | 重点演进与迁移提示 |
| --- | --- |
| `v0.16.0` | `/livez` 与 `/readyz` 探针、`PreStopDelay`、`AdminListen` 管理端口隔离、TLS/mTLS、`logger` 桥接 `log/slog`、独立 `metrics` 注册表与 `NewHTTPMetrics`、OpenTelemetry `middleware.Tracing`、Redis 多副本幂等 `RedisStore`、HTTP 防护中间件（`CORS`/`SecurityHeaders`/`RateLimit`/`MaxInFlight`/`RequestTimeout`/`MaxBodyBytes`）、`db/migrate` 与 `cli`/`proxy`/`stream`/`Asset`/`Debug` 加固 |
| `v0.15.0` | `conf.Watch` K8s `..data` 防抖热更新、`HTTPHost` 业务段热更新与 `RunWithShutdownHook`、`auth.Identity` 同步请求 `context.Context`、`sdk.Client` 自动透传 `Request-ID` 与幂等条件重试、SQL 参数脱敏日志、Redis 懒连接选项、选主指数退避与 `DummyAppElection` 对齐（**迁移**：已取消 `ctx` 传给 `DummyAppElection.Run` 返回 `context.Canceled`） |
| `v0.14.0` | 新增 `Apache-2.0` `LICENSE`；`RunMode = "release"` 禁止隐式/假鉴权（**迁移**：生产必须传入显式鉴权 handler）；加固多值身份头、`RequestID`/`Recovery`、DSN 安全构造与 `orderby` 白名单 |
| `v0.12.0`–`v0.13.0` | 新增 `middleware.HTTPAuthorization` 与服务间 `auth.InternalPropagatedIdentity`；**不兼容迁移（`v0.12.0`）**：组织标识统一重命名为租户（`x-forge-tenant-id`、`KeyTenantID`、`FakeAuthConfig.TenantID` 等），移除 `QueryOrgID` |
| `v0.10.0`–`v0.11.0` | `db.Transaction` 隔离级别与 PG/MySQL 死锁重试；`redis.NewRedisLazy` 与重导出驱动类型 |
| `v0.5.0`–`v0.9.0` | `app.Run` 统一应用入口（`v0.5.0`）；Go `1.27` 与标准库 H2C；接口收敛与 `AuthWithAuthCenter` 移除（`v0.6.0`）；`binder.All` 与 `defaults.Set` 请求契约（`v0.7.0`）；SDK `sdk.Complete` 与 `CommandLine` 实例（`v0.8.0`）；Swagger 注释与多模块生成器对齐（`v0.9.0`） |
| `v0.2.0`–`v0.4.0` | [设计审查](docs/server-go-design-review.md) 修复、配置密文升级为 AES-GCM + `SERVER_SECRET_KEY`（`v0.2.0`）、`stream.NewSSE` 与 `make check`（`v0.3.0`）、Bearer 凭据与严格 JSON 绑定（`v0.3.1`）、PostgreSQL 迁移至 `pgx/v5`（`v0.4.0`）；公开仓库清理私有日志/许可证 SDK 与达梦驱动并确立现有公开依赖边界 |

## 开源协议

本项目基于 [Apache-2.0](LICENSE) 协议开源。
