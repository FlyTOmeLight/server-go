# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

@AGENTS.md

## 常用开发命令

### 完整检查（每次框架修改后必须执行）

```bash
make check
```

验证根模块、四个独立示例模块、全部可运行示例及生成项目的格式、测试、vet、并发竞态检测，以及实际启动、HTTP/SDK/CLI 请求与退出。数据库检查会创建隔离的 PostgreSQL 17 和 MySQL 8.4 容器。

### 单模块检查

```bash
# 根模块（需要 CGO 支持 SQLite）
CGO_ENABLED=1 go test ./...
go vet ./...

# 独立示例模块（各有 go.mod）
cd examples/example-hello-world  # 或 example-service1/2、example-combine-service
CGO_ENABLED=1 go test ./...
go vet ./...
```

### 代码格式

```bash
# 格式化和检查（推送钩子要求）
gorgeous ./...
gorgeous -d ./...  # 仅检查，不修改
```

### 运行示例

```bash
# 最小示例（无需数据库）
cd examples/minimal-service
make run  # 或 go run ./cmd/server

# 其他示例
cd examples/example-hello-world
make run
make build
make check
```

### 代码生成

```bash
python3 scripts/gen_code.py \
  --name MyService \
  --modules Item \
  --go-mod github.com/your-org/my-service \
  --output ../generated
```

## 架构概览

### 核心模块职责

- **app**: 应用入口（v0.5.0+），`app.Run(ctx, options, build)` 统一配置、日志、HTTP 运行和资源清理
- **host**: HTTPHost 管理服务生命周期，`RegisterRoutes` 安装路由，`Start(ctx)` 启动任务，`Stop(ctx)` 取消并等待
- **server**: Bootstrap 负责失败回滚与逆序关闭
- **conf**: 配置管理器，支持热更新、占位符解析和 AES-GCM 加密
- **db**: 数据库连接、事务（支持显式隔离级别、只读、context 取消和重试）
- **logger**: 基于 Zap + Lumberjack 的日志，保留 Request ID 和级别过滤
- **auth**: 身份认证（Bearer tokens、可信网关头）和授权（代理、模拟）
- **middleware**: 请求 ID、幂等、许可证等中间件
- **pkg**: HTTP 类型封装（`core.Engine`、`core.Context` 等）和 binder
- **sdk**: 客户端构造和错误解码
- **cli**: 命令行框架，支持配置、Apply 注册和资源管理
- **stream**: SSE 流式响应管理

### 业务项目组织

业务项目采用 `cmd` + `internal` 布局：

- `cmd/server`、`cmd/cli`: 服务端和 CLI 入口
- `service/`: 可组合的公开服务构造
- `internal/app/<领域>/`: 私有业务逻辑
- `internal/table/`: 数据模型
- `conf/server/`: TOML 配置
- `sdk/go`、`internalsdk`: 对外和内部客户端

详见 [docs/go-project-layout.md](docs/go-project-layout.md)。

### 请求处理流程

1. HTTP 请求进入，中间件链处理（请求 ID、日志、Recovery、鉴权、幂等等）
2. Controller 使用 `binder.Bind(c, req).JSON()` 或 `binder.All()` 绑定和校验参数
3. 静态默认值通过 `defaults.Set(&req)` 在绑定前预填
4. Service 处理业务逻辑，返回错误
5. Controller 调用 `serrors.AbortWithError(ctx, err)` 输出错误响应

### 配置与资源交付

业务声明类型化配置结构体，`app.Run` 一次解析全部字段。装配函数通过 `runtime.Initialize(ctx, modules...)` 初始化资源，通过 `runtime.AddService(svc)` 交付服务。资源按交付顺序逆序关闭。

### 示例索引

- **minimal-service**: 仅需 HTTP 和配置
- **postgresql-service**: PostgreSQL CRUD、请求校验、默认值与 Go SDK
- **example-hello-world**: SQLite CRUD、Go SDK 与 CLI（生成器模板）
- **example-service1**: REST 数据集 CRUD、SDK 与 CLI
- **example-service2**: 无状态 HTTP 服务与公开构造函数
- **example-combine-service**: 组合三个服务、共享 SQLite 与组合 CLI
- **streaming-service**: SSE、取消传播、流式 SDK
- **example-leader-election**: 独立 worker 与 leader 任务

所有示例遵循同一套目录、配置、context 和生命周期约定，详见 [examples/README.md](examples/README.md)。
