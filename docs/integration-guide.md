# 业务项目接入与能力复用指南

本文档面向接入 `github.com/FlyTOmeLight/server-go` 的业务服务，串联**工程创建、生产配置、生命周期装配、中间件顺序、全栈能力复用及 Kubernetes 上线规范**，避免各项目重复造轮子。

- 框架能力总览与版本记录见 [README](../README.md)
- 目录职责、分层规范、SDK 与 Swagger 约定见 [Go 项目组织约定](go-project-layout.md)
- 配置占位符与加解密细则见 [conf 占位符文档](../conf/README.md)
- 数据库事务与死锁重试语义见 [db 事务文档](../db/README.md)

---

## 1. 能力速查表（避免重复造轮子）

动手自研公共能力前，先确认框架是否已内置对应包与入口：

| 业务需求 | 直接复用的框架包 | 核心入口 / 能力 |
| --- | --- | --- |
| 应用总控启动、配置热更新监听与优雅停机 | `app` / 根包 `server` | `app.Run`、`app.Runtime`、`server.NewBootstrap`、`server.Module` |
| 分环境 TOML 加载、环境变量/文件/密文占位符、防抖监听 | `conf` | `conf.Manager`、`Load`、`Watch`（支持 K8s `..data`）、`EncryptSecret` |
| HTTP 托管、健康探针、管理端口分离、TLS/mTLS、H2C | `host` / `httpintegration` | `HTTPHost`、`RegisterHealthCheck`、`AdminListen`、`InstallWithAuth`、`InstallDiagnostics` |
| 网关可信头、Bearer Token、标准 `net/http` 鉴权接入、内部透传、代理 | `auth` / `middleware` | `TrustedIdentityHeaders`、`BearerTokens`、`HTTPAuthorization`、`InternalPropagatedIdentity`、`Impersonate`、`auth.IdentityFromContext` |
| CORS、安全响应头、限流、并发上限、请求超时、请求体限制 | `middleware` | `CORS`、`SecurityHeaders`、`RateLimit`、`MaxInFlight`、`RequestTimeout`、`MaxBodyBytes` |
| POST 接口防重放（单机有界 / 多副本分布式） | `idempotence` / `middleware` | `idempotence.NewWithOptions`、`idempotence.NewRedisStore`、`middleware.Idempotent`、`middleware.IdempotentWithStore` |
| 统一错误模型、HTTP/API 错误码转换、`%w` 包装 | `errors` | `errors.NewError`、`Abort`、`AbortWithError`、`ErrorHandler`、`Wrap`、`WithMessage` |
| 分级分流日志、请求/链路 ID 自动注入、标准库 `slog` 桥接 | `logger` | `NewLogger`、`FromContext`、`WithContext`、`(*Logger).With`、`SlogHandler`、`Slog` |
| Prometheus 指标隔离注册表、低基数路由模板指标 | `metrics` | `Metrics`、`NewRegistry`、`HandlerFor`、`NewHTTPMetrics` |
| OpenTelemetry 服务端 span 与出站链路传播 | `middleware` / `sdk` | `middleware.Tracing`（`traceparent` 提取）+ `sdk.Client` 自动注入 |
| 结构化参数绑定、1 MiB JSON 严格解析、常用正则校验、分页参数 | `pkg/binder` / `sdk` | `binder.Bind(ctx, &req).All()`、`BindJSON`、`QueryLimit`、`PagingRequest`、`DBOrderWithAllowlist` |
| SSE 实时事件流与写超时控制 | `stream` | `stream.NewSSE`、`Send`、`SendWithID`、`Heartbeat` |
| HTTP 反向代理与 Swagger/Console FE-OpenAPI 适配 | `proxy` / `console` / `swagger` | `proxy.NewReverseProxy`、`console.RegisterConsoleAPI`、`swagger.SwaggerWithPrefix` |
| 受保护/公开静态文件托管（防目录遍历/点文件泄露） | `middleware` | `middleware.Asset` |
| MySQL / PostgreSQL / SQLite / ClickHouse / Kingbase / GaussDB | `db` | `db.Module`、脱敏 SQL 日志、`db.Transaction`（含死锁重试）、`UTCTime`、`db.NewError` |
| 版本化 SQL / Go 数据库迁移 | `db/migrate` | `LoadFS`、`New`、`NewFromGORM`、`Up`、`Down`、`Status` |
| MongoDB 客户端、BSON 时间序列化与错误映射 | `db/mongodb` | `mongodb.Module`、`NewClientWithConfig`、`mongodb.Time`、`mongodb.NewError`、`GetFindOptions` |
| Redis 连接、懒加载订阅与断线重连 | `redis` | `NewRedis`、`NewRedisLazy`、`redis.Module{Lazy: true}`、`XReadArgs`、`redis.Nil` |
| Dapr 事件订阅发现与连接复用发布 | `pubsub` | `NewRegistry`、`Subscribe`、`OnEventWithGroup`、`NewPublisher`、`PublishEvent` |
| 周期任务调度、运行时间窗与协程池异常恢复 | `cron` | `NewTaskManager`、`NewPeriodTask`、`RunPeriod`、`RegisterSyncHandler`、`SetErrorHandler` |
| Kubernetes 租约选主与单机调试实现 | `pkg/election` | `NewLeaderElection`、`NewK8sLeaderElection`、`NewDummyAppElection`、`LookupLocalIP` |
| 构建版本端点与服务端版本查询 | `version` | `Version()`、`GetVersionByURL`、`GetVersionByURLContext` |
| 对外 Go SDK 与内部服务 SDK 客户端 | `sdk` / `internalsdk` | `sdk.NewClientWithConfig`（幂等重试/`Complete`/`DecodeError`）、`internalsdk.Wrap` |
| 服务配套 Cobra CLI（`apply` / `config` / `encrypt` / 表格输出） | `cli` | `NewCommandLineWithName`、`cli.Client`、`RegisterCommandApplyHandler`、`WriteFormatted` |
| 无协程泄漏内存缓存、ID 生成、结构体解码、通用类型工具 | `pkg/*` / `naming` / `constants` | `pkg/cache`、`NewID`/`NewUUID`/`RandomString`/`NewHashID`、`ParseFile`、`StructDecode`、`pkg/datatypes.StringMap`、`pkg/values`、`pkg/compare`、`naming`、`constants` |

---

## 2. 两种接入路径

### 路径 A：新建业务服务

按业务形态选择起点（均不要把业务代码放进 `server-go` 仓内）：

1. **快速生成全套 CRUD + SDK + CLI + Swagger 骨架**（以 `example-hello-world` 为模板）：

   ```sh
   python3 scripts/gen_code.py \
     --name OrderService \
     --modules Order Item \
     --go-mod github.com/your-org/order-service \
     --output ../order-service
   cd ../order-service/order-service
   go mod tidy
   CGO_ENABLED=1 go build ./cmd/server && CGO_ENABLED=0 go build ./cmd/cli
   ```

2. **手工从合适示例裁剪起步**：
   - 无状态/自定义 HTTP：复制 [minimal-service](../examples/minimal-service/README.md)（`CGO_ENABLED=0` 构建）
   - PostgreSQL 生产 CRUD + RESTful 校验默认值：参考 [postgresql-service](../examples/postgresql-service/README.md)（`CGO_ENABLED=0` 构建）
   - SSE 流式接口：参考 [streaming-service](../examples/streaming-service/README.md)
   - 纯后台选主 Worker（无需 HTTP）：参考 [example-leader-election](../examples/example-leader-election/README.md)
   - 单进程组合多个业务域共享数据库与端口：参考 [example-combine-service](../examples/example-combine-service/README.md)

**依赖与 CGO 约定**：
- 只有直接配置 `Driver = "SQLite"` 的服务或测试才需要 `CGO_ENABLED=1`；MySQL、PostgreSQL、ClickHouse、Kingbase、GaussDB、MongoDB、Redis 或无状态服务均可用 `CGO_ENABLED=0` 纯静态编译。
- 多仓联调未发版改动时可在本地临时使用 `go mod edit -replace=github.com/FlyTOmeLight/server-go=/path/to/server-go`，合并上线前**必须移除本地 `replace`** 并引用已发布的版本标签。

### 路径 B：存量 Go/Gin 项目接入 `server-go`

存量 Gin 服务只需把原有路由注册封装为 `host.Service`（实现 3 个方法），其余配置、探针、优雅退出、统一错误中间件全部由 `app.Run` 接管：

```go
package legacyadapter

import (
	"context"

	"github.com/FlyTOmeLight/server-go/host"
	core "github.com/FlyTOmeLight/server-go/pkg"
)

type LegacyService struct {
	// 注入已构造好的 DB / Redis / 业务依赖
}

var _ host.Service = (*LegacyService)(nil)

func (s *LegacyService) RegisterRoutes(router *core.RouterGroup) error {
	// 注意：router 已带 config.HTTPServer.APIBasePath 前缀，
	// 不要在业务内部再重复安装 RequestID / Recovery / ErrorHandler 或 /healthz。
	router.GET("/v1/legacy/orders", s.listOrders)
	return nil
}

func (s *LegacyService) Start(ctx context.Context) error {
	// 启动后台 goroutine / 消费者；需响应 ctx 取消，不要复用初始化时的 initCtx。
	return nil
}

func (s *LegacyService) Stop(ctx context.Context) error {
	// 等待后台 goroutine 退出并返回错误；超时/失败由框架按序处理。
	return nil
}
```

迁移存量 handler 时重点对齐两件事：
1. 删除业务里自行注册的 Gin `Default()`、`RequestID`、`Recovery`、CORS 或裸 `c.JSON(500, ...)`，改为复用 `middleware.*` 与 `errors.AbortWithError`。
2. 异步或 DB/SDK 调用一律传 `c.Request.Context()`，不要将 `*core.Context` 直接抛入后台 goroutine（Gin 会复用 `*core.Context`）。

---

## 3. 生产配置模板与热更新边界

默认配置文件路径为工作目录下的 `conf/server/config.toml`，生产建议通过环境变量 `SERVER_CONFIG_PATH` 显式挂载，并结合 `${env:...}`、`${secret:...}`、`${file:...}` 避免明文密钥落盘（详见 [conf/README.md](../conf/README.md)）：

```toml
AppName = "order-service"
AppCode = 101                    # 非零业务应用编码，会组合到错误码 StatusCode 中
RunMode = "release"              # debug | test | release；release 禁止 FakeAuth
DeployMode = "Cloud"             # Cloud | Edge
Services = ["order"]

[HTTPServer]
Listen = "0.0.0.0:8080"
AdminListen = "127.0.0.1:9090"   # 可选：启用后 pprof/metrics/swagger 只挂在管理端口（明文 HTTP）
APIBasePath = ""                 # 统一业务路由前缀，如 "/order"
AuthType = "AuthCenter"          # Fake | AuthCenter；release 下需配对显式 authenticate handler
ReadTimeout = 20000              # 毫秒，0 用默认 20000
ReadHeaderTimeout = 5000         # 毫秒，0 用默认 5000
WriteTimeout = 10000             # 毫秒，0 用默认 10000（SSE 端点会在 stream.NewSSE 中清除单次前写超时）
IdleTimeout = 10000              # 毫秒，0 用默认 10000
PreStopDelay = 3000              # 毫秒，停机前先把 /readyz 标为 503 并等待网关摘流
MaxHeaderBytes = 100000
TrustedProxies = ["10.0.0.0/8"]  # 仅信任入口网关网段；为空则不信任任何代理头
PprofEnable = false
MetricsEnable = true
SwaggerEnable = false
SwaggerDocPrefix = "Order"       # 多服务实例隔离 Swagger 文档名
UseH2C = false                   # 仅纯明文 HTTP/2（prior knowledge）场景开启；与 TLS 互斥

[HTTPServer.TLS]
Enable = false                   # 开启后主监听使用 TLS 1.2+，AdminListen 仍保持明文 HTTP
CertFile = "/etc/tls/tls.crt"
KeyFile = "/etc/tls/tls.key"
ClientCAFile = ""                # 非空时开启 mTLS（tls.RequireAndVerifyClientCert）

[DB]
Driver = "PostgreSQL"            # MySQL | SQLite | PostgreSQL | ClickHouse | Kingbase | GaussDB
DSN = "postgres://${env: DB_USER}:${secret: v1:<加密密文>}@${env: DB_HOST}:5432/order?sslmode=require"
MaxIdleConns = 10
MaxOpenConns = 50
ConnMaxIdleTime = 300            # 秒
ConnMaxLifeTime = 1800           # 秒
AutoMigrate = false              # 生产推荐走 db/migrate，不使用 GORM AutoMigrate

[Redis]
Addrs = ["${env: REDIS_ADDR:127.0.0.1:6379}"]
Password = "${file: /run/secrets/redis-password}"
DB = 0
```

**热更新边界（`conf.Manager.Watch` 已内置于 `app.Run`）**：
- **自动生效（原子重建并热切换路由）**：`RunMode`、`Feature`、`FakeAuth`，以及 `HTTPServer` 中除监听与传输外的配置（`APIBasePath`、`PprofEnable`、`MetricsEnable`、`SwaggerEnable`、`SwaggerDocPrefix`、`Trace`、`AuthType`、`ConsoleEnable`、`ConsoleProxyNames`、`AltoHandlerAPIError`、`Assets`、`AllowAPIPathNoHandlerAPIError`）。
- **必须重启进程（准备阶段返回 `host.ErrRestartRequired` 并保留旧快照）**：`AppName`、`AppCode`、`DeployMode`、`Services`，以及 `Listen`、`AdminListen`、`ReadTimeout`、`ReadHeaderTimeout`、`WriteTimeout`、`IdleTimeout`、`PreStopDelay`、`MaxHeaderBytes`、`UseH2C`、`TLS`、`TrustedProxies`。
- **基础设施连接不隐式重建**：`DB`、`Redis`、`MongoDB`、`Publisher` 等连接生命周期由装配代码管理，变更后需重启进程。

---

## 4. 启动装配与中间件挂载顺序

### 4.1 入口与服务装配模板

**`cmd/server/main.go`**（仅处理进程信号、退出码与顶层 `Setup` 装配）：

```go
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/FlyTOmeLight/server-go/app"
	"your-module/service"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	options := app.Options{
		Setup: service.SetupHTTP,
	}
	if err := app.Run(ctx, options, service.Build); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
```

**`service/build.go`**（先声明强类型 `Config`，再用 `runtime.Initialize` 注册基础设施模块，最后 `runtime.AddService`）：

```go
package service

import (
	"context"
	"embed"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/FlyTOmeLight/server-go/auth"
	"github.com/FlyTOmeLight/server-go/conf"
	"github.com/FlyTOmeLight/server-go/db"
	"github.com/FlyTOmeLight/server-go/db/migrate"
	"github.com/FlyTOmeLight/server-go/httpintegration"
	"github.com/FlyTOmeLight/server-go/idempotence"
	"github.com/FlyTOmeLight/server-go/metrics"
	"github.com/FlyTOmeLight/server-go/middleware"
	core "github.com/FlyTOmeLight/server-go/pkg"
	"github.com/FlyTOmeLight/server-go/redis"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

type Config struct {
	DB    db.Config
	Redis redis.Config
}

func Build(ctx context.Context, runtime *app.Runtime, cfg *Config) error {
	database := &db.Module{Config: &cfg.DB}
	cacheRedis := &redis.Module{Config: &cfg.Redis, Lazy: true}
	if err := runtime.Initialize(ctx, database, cacheRedis); err != nil {
		return err
	}

	// 启动期版本化 SQL/Go 迁移（替代裸 GORM AutoMigrate）
	migrations, err := migrate.LoadFS(migrationFS, "migrations")
	if err != nil {
		return err
	}
	runner, err := migrate.NewFromGORM(database.DB, migrate.WithMigrations(migrations...))
	if err != nil {
		return err
	}
	if err := runner.Up(ctx); err != nil {
		return err
	}

	idemStore := idempotence.NewRedisStore(cacheRedis.Redis, idempotence.RedisStoreOptions{
		Prefix:  "order-idem",
		TTL:     24 * time.Hour,
		LockTTL: time.Minute,
	})
	svc, err := NewOrderService(ctx, database.DB, idemStore)
	if err != nil {
		return err
	}
	return runtime.AddService(svc)
}
```

### 4.2 `HTTPHost` 默认已装能力（禁止重复挂载）

每次构建路由器时，`HTTPHost` 已经自动完成以下工作：
1. 绑定 `TrustedProxies`，开启 `HandleMethodNotAllowed` 与 `ContextWithFallback`。
2. 全局前置中间件：`logger.WithContext` 注入、设置 `"server.AppCode"` / `"server.Debug"`、`middleware.RequestID()`、`middleware.RequestLog()`、`middleware.Recovery()`、`errors.ErrorHandler(config)`。
3. 全局兜底：`NoRoute` 返回 `404 APINotFound`，`NoMethod` 返回 `405 MethodNotAllowed`。
4. 探针与版本端点：`/ping`、`/healthz`、`/livez`、`/readyz` 与主路由 `/version`，并自动调用 `binder.RegisterValidator()`。

### 4.3 `SetupHTTP` 中间件正确分层顺序

因为 `HTTPHost` 会按配置热更新重建路由，`SetupHTTP` **只负责注册路由与中间件，不要在 `SetupHTTP` 里新建 Controller 或连接池**；同时注意不同层级中间件的位置：

```go
// 指标收集器应在装配期只注册一次，避免配置热更新重建路由时重复注册。
var httpMetrics, _ = metrics.NewHTTPMetrics(prometheus.DefaultRegisterer, "order")

func SetupHTTP(engine *core.Engine, cfg *conf.Config) error {
	// 第 1 层：全链路观测与边缘防护（位于鉴权之前）
	engine.Use(
		middleware.Tracing("order-service", middleware.WithSkipPaths("/livez", "/readyz", "/healthz", "/ping", "/metrics")),
		httpMetrics.Middleware(),
		middleware.CORS(middleware.CORSConfig{
			AllowOrigins: []string{"https://console.example.com"},
			AllowMethods: []string{"GET", "POST", "PUT", "PATCH", "DELETE"},
			AllowHeaders: []string{"Authorization", "Content-Type", "Request-ID", "Idempotency-Key"},
		}),
		middleware.SecurityHeaders(),
		middleware.MaxBodyBytes(4<<20),        // 防超大请求体耗尽内存
		middleware.RequestTimeout(30*time.Second),
		middleware.MaxInFlight(2000),
		middleware.RateLimit(middleware.RateLimitConfig{RequestsPerSecond: 100, Burst: 200}),
	)

	// 第 2 层：框架集成（Debug 脱敏日志、AdminListen="" 时的诊断路由、/feature、公开静态资源、Console 代理、入口鉴权、受保护静态资源）
	authHandler, err := auth.TrustedIdentityHeaders(verifyGatewaySignature)
	if err != nil {
		return err
	}
	if err := httpintegration.InstallWithAuth(engine, cfg, authHandler); err != nil {
		return err
	}

	// 第 3 层：鉴权后中间件（可选：租户级限流、License 校验、内部服务透传 / 代理）
	// internalProp, _ := auth.InternalPropagatedIdentity(checkCallerService)
	// engine.Use(internalProp)
	return nil
}
```

**两个关键集成注意事项**：
- **幂等中间件位置**：`middleware.Idempotent` / `middleware.IdempotentWithStore` **不能**全局 `engine.Use`，必须挂在具体写接口路由链的**最后一个** handler（鉴权 + 业务权限校验之后，见第 5 节）。
- **自定义 Prometheus 注册表 vs `AdminListen`**：
  - 若使用全局 `prometheus.DefaultRegisterer`（如上例 `httpMetrics`），直接设置 `MetricsEnable = true`，启用 `AdminListen` 后 `/metrics` 自动仅在管理端口暴露，不暴露在公网业务端口。
  - 若使用完全隔离的 `reg := metrics.NewRegistry()` 且不启用 `AdminListen`，请将配置中 `MetricsEnable = false`，然后手工挂载 `engine.GET("/metrics", metrics.HandlerFor(reg))`（避免与 `httpintegration.InstallDiagnostics` 冲突）。
- **自定义 `/readyz` 依赖检查**：`h.RegisterHealthCheck("postgres", checkFn)` 位于 `*host.HTTPHost`。若需在 `app.Run` 之外显式挂自定义就绪探针，可按 `conf.NewManager` + `logger.NewLogger` + `server.NewBootstrap` + `host.NewHTTPHost` 装配后在 `h.Run(ctx)` 前调用 `h.RegisterHealthCheck`。

---

## 5. 鉴权、身份传播与安全防护

### 5.1 生产鉴权模式选择

`RunMode = "release"` 下未提供显式 handler 时，`auth.Use` 与 `httpintegration.Install` 会在构建路由阶段直接失败。按入口场景选择其一：

| 入口场景 | 推荐组件 | 关键行为 |
| --- | --- | --- |
| API 网关统一鉴权后转发 | `auth.TrustedIdentityHeaders(verify)` | `verify` 验签网关请求头控制权后读取 `x-forge-user-id`/`x-forge-tenant-id`/`x-forge-project-id`，拒绝同名多值头；不写 `KeyImpersonateOrigin*` |
| 静态服务账号 / Token 鉴权 | `auth.ParseBearerTokens` + `auth.BearerTokens` | 要求每条 token ≥ 24 字符，预计算 SHA-256 摘要进行全量常量时间比对 |
| 复用第三方标准 `net/http` 鉴权中间件 | `middleware.HTTPAuthorization(mw, publish)` | 保留 Gin 包装后的 `ResponseWriter`（支持 Flush/Hijack/状态码），并同步发布身份到 `core.Context` 与标准 `context.Context` |
| 内部服务透传已验证用户身份 | 上游 `internalsdk.Wrap` + 下游 `auth.BearerTokens` + `auth.InternalPropagatedIdentity` | 读取 `x-impersonate-target-*`，同时写入 `Key*` 与 `KeyImpersonateOrigin*`（`KeyAuthMode = "Internal"`） |
| 管理员 / 服务代表他人执行操作 | `auth.Impersonate(authorize)` | 保留原操作者于 `KeyImpersonateOrigin*`，将目标身份写入 `Key*`（`KeyAuthMode = "Impersonate"`） |

框架所有鉴权中间件都会调用 `auth.SyncIdentityToRequestContext(c)`，在纯 Go 业务层或存储层可直接通过 `id, ok := auth.IdentityFromContext(ctx)` 获取结构化 `auth.Identity`（无需依赖 `*gin.Context`）。

### 5.2 接口幂等（单机与多副本）

`middleware.Idempotent` / `middleware.IdempotentWithStore` 仅在同时满足以下条件时生效：`Method == POST`、`Content-Type == application/json`、请求携带非空 `?clientToken=` 或请求头 `Idempotency-Key`、且经过鉴权存在 `KeyUserID`。

- **单机有界模式**：`idempotence.New(24*time.Hour)`（默认 10,000 条、1 MiB 缓存响应体上限）或 `idempotence.NewWithOptions(opts)`（注意 `Options.MaxBodyBytes <= 0` 时不限制缓存体大小）。
- **多副本 Redis 模式**（推荐生产多 Pod 服务）：`idempotence.NewRedisStore(client, idempotence.RedisStoreOptions{...})`，通过 `Prefix:SHA256(TenantID, UserID, ProjectID, Method+" "+Path, Token)` 隔离键空间。**注意**：`LockTTL` 必须大于业务接口最长执行耗时。重放时自动过滤 `Date`、`Set-Cookie` 与旧 `Request-ID`，写入当前请求的 `Request-ID`。

```go
orders.POST("",
	requirePermission("order:create"),
	middleware.IdempotentWithStore(s.idemStore, s.controller.Create),
)
```

---

## 6. HTTP 契约、SSE、反代与静态资源

### 6.1 Controller 绑定、默认值与错误返回

```go
func (c *Controller) Create(ctx *core.Context) {
	var req orderv1.CreateOrderRequest
	if err := defaults.Set(&req); err != nil {
		serrors.AbortWithError(ctx, err)
		return
	}
	if err := binder.Bind(ctx, &req).All().Error(); err != nil {
		serrors.AbortWithError(ctx, err)
		return
	}
	resp, err := c.service.Create(ctx.Request.Context(), &req)
	if err != nil {
		serrors.AbortWithError(ctx, err)
		return
	}
	ctx.JSON(http.StatusCreated, resp)
}
```

- **可用绑定器**：`binder.Bind(ctx, &req)` 支持 `.All()`（合并 body + query + uri，优先级 URI > query > body）、`.JSON()`（严格单 JSON 文档、拒绝未知字段、1 MiB 限制）、`.Query()`、`.Uri()`、`.Form()`、`.MultiForm()`、`.Header()`；另有快捷函数 `binder.BindJSON(ctx, &dst)` 与 `binder.QueryLimit(ctx, defaultLimit, maxLimit)`。
- **内置 validator 规则**（由 `HTTPHost` 自动调用 `binder.RegisterValidator()`）：`name`、`localname`、`orderby`（严格列名 + `:asc/:desc` 校验）、`id`（支持前缀参数如 `binding:"id=ord"`）、`alphanumeric`、`lowercase_alphanumeric`、`category`、`categories`、`tags`。
- **错误规范**：框架 `Error` 会对 500 与未知错误在非 Debug 模式自动脱敏为 `ErrorInternalError`，注入 `RequestID` 与 `<AppCode><HTTPStatus>` 组合状态码，且通过 `ErrorHandler` 兼容 `AltoHandlerAPIError`。

### 6.2 SSE、反向代理、Swagger 与静态资源

- **SSE 流式推送（`stream`）**：

  ```go
  sse, err := stream.NewSSE(ctx.Writer, 5*time.Second)
  if err != nil {
      serrors.AbortWithError(ctx, err)
      return
  }
  if err := sse.SendWithID("1", "message", payload); err != nil {
      return
  }
  _ = sse.Heartbeat() // 发送 ": heartbeat\n\n" 保活
  ```

  `NewSSE` 会在初始化时清除外层 `http.Server.WriteTimeout`，每次 `Send` / `SendWithID` / `Heartbeat` 前设置单次写入超时并立即 `Flush()`，防止空闲等待误触发超时断开。
- **通用反向代理（`proxy`）**：`proxy.NewReverseProxy("http://upstream:8080")` 校验绝对 URL、自动设置 `X-Forwarded-*` 与 `Host`，复用连接池，失败返回 502。
- **多服务 Swagger 实例（`swagger`）**：通过配置 `HTTPServer.SwaggerDocPrefix = "Order"`（或调用 `swagger.SwaggerWithPrefix("Order")`），避免多服务进程覆盖包级 `swagger.DocPrefix`。
- **静态资源托管（`middleware.Asset`）**：在配置中通过 `HTTPServer.Assets` 分别声明公开（`AuthEnable = false`）或需鉴权后访问（`AuthEnable = true`）的静态目录；内置基于 `os.OpenRoot` 的路径越权防护、自动拒绝 `.` 开头隐藏文件及目录列表浏览，并设置 `X-Content-Type-Options: nosniff`。

---

## 7. 可观测性（日志、指标、追踪）

- **结构化日志与标准库 `slog`**：
  - `logger.FromContext(ctx)` 自动提取上下文中的 `requestId`、`traceId`、`spanId`。
  - 需要结构化派生子 logger 或对接第三方使用 `log/slog` 的库时，直接复用 `appLog.With("component", "worker")`、`appLog.SlogHandler()` 或 `appLog.Slog()`（所有 `borrowed` 子 logger 共享父输出文件句柄）。
  - 需要跳过健康检查刷屏或输出请求身份字段时，可在 `SetupHTTP` 或子路由中配合 `middleware.RequestLogWithOptions(middleware.RequestLogOptions{SkipPaths: []string{"/livez", "/readyz"}, IncludeIdentity: true})`。
- **Prometheus 指标（`metrics`）**：`metrics.NewRegistry()` 创建独立注册表，`metrics.NewHTTPMetrics(reg, "service_prefix")` 基于 Gin 路由模板（`c.FullPath()`，未匹配归一为 `"unmatched"`）采集低基数 QPS、耗时直方图与并发请求数。
- **OpenTelemetry 链路追踪（`middleware.Tracing` + `sdk`）**：服务端安装 `middleware.Tracing("service-name")` 提取入站 `traceparent` 并开启 `SpanKindServer` span；客户端使用框架 `sdk.Client` / `internalsdk.InternalClient` 调用下游时会自动注入追踪头与 `Request-ID`。

---

## 8. 数据存储、消息、定时任务与 Worker

### 8.1 数据库（SQL / 迁移 / MongoDB）与 Redis

- **SQL (`db` & `db/migrate`)**：
  - 支持 `MySQL`、`SQLite`、`PostgreSQL`、`ClickHouse`、`Kingbase`、`GaussDB`。默认使用脱敏 `parameterizedGORMLogger`，SQL 参数不打印明文，>200ms 查询自动记录 `Warn` 日志。
  - 列表排序使用 `req.DBOrderWithAllowlist(allowedCols, "created_at")` 防 ORDER BY 注入；模糊搜索关键字转义使用 `db.ReplaceSpecialKeyword`。
  - 事务统一走 `db.Transaction`（详见 [db/README.md](../db/README.md)），支持显式隔离级别、只读标记及 PostgreSQL/GaussDB（`40001`/`40P01`）/ MySQL（`1213`）死锁重试。
  - 错误转换：`db.NewError(err)` 将唯一冲突、记录不存在、非法入参映射为框架标准 `serrors.Error`（保留原始 `%w` 错误链）。
  - 表结构变更使用 `db/migrate`（见第 4.1 节示例），支持 `embed.FS` 读取 `0001_init.up.sql` / `.down.sql`、SHA-256 校验和核对以及 `Up` / `Down(ctx, steps)` / `Status`。
- **MongoDB (`db/mongodb`)**：
  - 从 `v0.17.0` 起已与根 `db` 包彻底解耦（纯 SQL 服务不再拉取 `mongo-driver` v1）。MongoDB 业务请使用 `mongodb.Module`（或 `mongodb.NewClientWithConfig`）、`mongodb.Time` 做 BSON 纳秒时间序列化、`mongodb.NewError(err)` 做错误转换以及 `(*mongodb.DB).GetFindOptions(req)` 构造有序分页排序。
- **Redis (`redis`)**：
  - 允许 Redis 晚就绪或自动重连的场景使用 `redis.Module{Config: &cfg.Redis, Lazy: true}` 或 `redis.NewRedisLazy`；包内直接重 export 了 `redis.XReadArgs` 与常量 `redis.Nil`，业务无需直接引入 `go-redis`。

### 8.2 消息（Dapr PubSub）、定时调度与选主 Worker

- **Dapr PubSub (`pubsub`)**：
  - 订阅注册：`reg := pubsub.NewRegistry()`，调用 `reg.OnEvent(engine, "order/created", cfgHandler, handler)` 或 `reg.OnEventWithGroup(...)`，最后挂载 `engine.GET("/dapr/subscribe", reg.Subscribe())`。
  - 高频事件发送：使用长连接池化客户端 `pub, err := pubsub.NewPublisher()`（应用退出时 `pub.Close()`），不要在热路径反复调用单次短连接 `pubsub.PublishEvent`。
- **定时调度（`cron`）**：
  - `tm, err := cron.NewTaskManager(10*time.Second, 4, true)`，通过 `RegisterSyncHandler` 同步 `cron.NewPeriodTask(task, runPeriods, policy, enabled)`，建议调用 `tm.SetErrorHandler(...)` 接管同步/运行报错；内置 `ants` 协程池 panic 捕获、时区感知时间窗校验与优雅退出等待。
- **选主与 Modbus**：
  - 后台高可用 Worker 使用 `pkg/election`：`NewLeaderElection(appName, ...)`（基于 Kubernetes `LeaseLock`，默认 15s 租约 / 10s 续约 / 2s 重试并带指数退避）；本地单进程开发显式传入 `NewDummyAppElection(id)`。
  - 工业协议服务可实现 `modbus.API` 并通过 `modbus.NewDefaultModbusService(addr, port, api)` + `host.NewModbusHost(svc).Run(ctx)` 托管。

---

## 9. SDK、内部客户端、CLI 与工具包复用

- **构建业务 SDK（`sdk` / `internalsdk`）**：
  - 使用 `sdk.NewClientWithConfig(&sdk.Config{...})` 复用连接池、代理验证、`Request-ID` 透传、OpenTelemetry 上下文注入与幂等条件重试（`Retry > 0` 时仅对 `GET`/`HEAD`/`OPTIONS`/`PUT`/`DELETE` 或携带 `Idempotency-Key`/`clientToken` 的请求重试 429/5xx）。
  - 请求调用后用 `sdk.Complete(result, err)` 收口处理网络与服务端错误（自动回填响应头 `Request-ID`）；非普通 JSON 失败可用 `sdk.DecodeError` 解码。
  - 服务间调用传递入站身份时使用 `internalsdk.NewInternalClientWithConfig` 与 `internalsdk.Wrap(client, ginCtx)`。
- **构建业务 CLI（`cli`）**：
  - `cmd := cli.NewCommandLineWithName("myctl")` 创建独立实例（不污染包级全局变量），`cmd.Append(...)` 注册业务子命令；命令内部通过 `client, err := cli.Client(c.Context())` 获取客户端；声明式资源创建通过 `cli.RegisterCommandApplyHandler` 接入内置 `apply -f/-d`。
  - 框架内置 `config get/set/unset`（`0600` 权限保护、敏感字段默认脱敏）、`encrypt --stdin`/管道密文生成、`version` 与多格式打印（`PrintJSON`、`PrintYAML`、`PrintTable`、`PrintCSV`、`PrintMarkdown`）。
- **基础工具包复用（`pkg/*` / `naming` / `version`）**：
  - **标识与文件**：`core.NewID("ord")`（8 位随机后缀）、`core.NewUUID()`、`core.RandomString`、`core.NewHashID`/`NewHashIDByObject`、`core.ParseFile`（自动识别 JSON/TOML/YAML）、`core.CreateFileIfNotExist`、`core.StructDecode`。
  - **内存缓存**：`pkg/cache.NewCache`（`sync.RWMutex` + 惰性清理，不泄漏后台 goroutine，支持 `DeleteByPrefix` 与幂等 `Close`）。
  - **数据库通用类型**：`pkg/datatypes.StringMap` 直接持久化 `map[string]string` 为 JSON 文本；`naming` 解析/校验分层资源名（`workspaces/<ws>/...`）。
  - **其他通用助手**：`pkg/compare`（`IsIncreasing` / 数字字符串友好 `Less`）、`pkg/values`（`GetValueOrDefault` / `GetFirstNonZeroValue` / `First`）、`pkg/safety`（`ValidateCommand` 注入初筛；注：同包 `CBCEncrypter`/`CBCDecrypter` 仅用于兼容旧加密协议，新密文务必使用 `conf.EncryptSecret` AES-GCM）、`version.GetVersionByURLContext`（查询远端服务 `/version`）。

---

## 10. Kubernetes 部署与上线自查清单

### 10.1 推荐探针与优雅下线配置

```yaml
env:
  - name: SERVER_CONFIG_PATH
    value: /etc/service/config.toml
  - name: SERVER_SECRET_KEY
    valueFrom:
      secretKeyRef:
        name: service-secret-key
        key: aes-key
ports:
  - name: http
    containerPort: 8080
  - name: admin
    containerPort: 9090
livenessProbe:
  httpGet:
    path: /livez
    port: http
  periodSeconds: 10
readinessProbe:
  httpGet:
    path: /readyz
    port: http
  periodSeconds: 5
terminationGracePeriodSeconds: 30
```

- **下线预算匹配**：`terminationGracePeriodSeconds`（如 30s）须大于 `PreStopDelay`（如 3s）+ HTTP 排空预算（5s）+ `app.Options.CloseTimeout` × 资源步数。
- **ConfigMap 热更新**：`conf.Manager.Watch` 已原生支持 Kubernetes ConfigMap `..data` 符号链接原子切换与 50ms 防抖；将配置文件挂入独立目录（不要挂载单文件 `subPath`），修改无需重启的可热更新字段可自动生效。

### 10.2 上线前自查清单

- [ ] `RunMode = "release"`，且 `httpintegration.InstallWithAuth` 已传入显式生产鉴权（未残留 `FakeAuth`）
- [ ] `HTTPServer.AdminListen` 已独立监听内网地址或仅在安全内网暴露 `/pprof`、`/metrics`、`/swagger`
- [ ] `go.mod` 中无指向本地相对路径的 `replace`，引用的 `github.com/FlyTOmeLight/server-go` 为正式发布标签
- [ ] 需要 SQLite 的二进制保留 `CGO_ENABLED=1`，无 SQLite 服务使用 `CGO_ENABLED=0` 构建
- [ ] 配置文件中的密码、私钥等通过 `${secret:...}` / `${env:...}` / `${file:...}` 注入，环境变量 `SERVER_SECRET_KEY` 已随 Secret 部署
- [ ] K8s `livenessProbe` 指向 `/livez`，`readinessProbe` 指向 `/readyz`（需要外部强依赖就绪判断时已注册 `HTTPHost.RegisterHealthCheck`）
- [ ] 需要跨副本防重放的写接口已挂接 `middleware.IdempotentWithStore`（`RedisStore` 且 `LockTTL` > 最大处理耗时），且位于鉴权之后、处理器之前
- [ ] 数据库变更已接入 `db/migrate`，多实例场景确保仅由单一迁移任务/所有者执行 `Up`
- [ ] 后台业务均通过 `c.Request.Context()` 传递取消与链路上下文，无跨请求引用 `*core.Context`
- [ ] 全量跑通 `gorgeous -d ./...`、`CGO_ENABLED=1 go test -race ./...` 与 `go vet ./...`
