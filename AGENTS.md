# server-go 项目规则

## 项目定位

本仓库维护可供其他 Go 项目引用的 Gin 服务框架。公开模块路径为 `github.com/FlyTOmeLight/server-go`。搭建业务项目时先读 [README.md](README.md)，从 `examples/minimal-service` 打通 HTTP 流程；需要数据库、SDK 和 CLI 时参考 `examples/example-hello-world`。

## 业务项目组织

新建业务服务、调整目录或编写 SDK 时，读取 [Go 项目组织约定](docs/go-project-layout.md) 的对应章节。采用 `cmd` + `internal`，按 Controller、Service、Table、SDK 的实际职责组织；框架接口和配置路径以当前实现为准。

参考其他业务项目时，只沉淀可复用的工程规则；本仓库文档不保留参考业务项目的名称、本地路径或专有实现细节。

## 框架与业务边界

- 框架根模块只放跨项目复用的能力。业务项目使用自己的 module path，把领域逻辑放在自己的 `internal/` 中，通过发布版本引用本框架。
- 当前工作树使用 `app.Run(ctx, options, build)` 统一创建配置与日志、监听配置、运行 HTTP 和清理资源。业务声明类型化配置，由 `app.Run` 按字段名一次解析后传给装配函数；同名 key 无需 tag，异名映射才使用 `mapstructure` tag。业务装配通过 `runtime.Initialize` 初始化资源，通过 `runtime.AddService` 交付服务；构造函数清理尚未交付的部分资源。可组合服务放在与 `cmd` 同级的公开 `service/`，私有业务留在 `internal/`。底层 `server.NewBootstrap` 负责失败回滚与逆序关闭，`host.NewHTTPHost` 装配 `host.Service`。`RegisterRoutes` 只安装路由，`Start(ctx)` 启动所属任务，`Stop(ctx)` 取消并等待任务。进程入口处理信号和退出码；`app.Run` 接管资源清理。应用入口从 v0.5.0 开始提供，正式消费使用发布版本。
- 配置使用 `conf.Manager` 实例和独立快照；应用入口一次解析业务配置结构体，装配代码使用类型化字段，资源通过构造函数注入。配置路径、占位符和动态更新范围见 [conf/README.md](conf/README.md)。
- HTTP 类型优先使用 `pkg/gin.go` 定义的 `core.Engine`、`core.Router`、`core.RouterGroup`、`core.Context`、`core.HandlerFunc`；未封装的 Gin 函数、常量和类型仍直接使用 Gin。
- HTTP 参数、响应和错误沿用框架现有契约。修改前检查 `pkg/gin.go`、`response.go`、`errors/errors.go` 及相关 SDK 调用方。
- SDK 请求的必填、长度、范围、枚举和格式校验优先使用 `binding` struct tag，通过 binder 返回参数错误；按输入来源声明 `json`、`form`、`uri` 标签。优先复用 validator 内置规则与 `pkg/binder`，跨项目通用且已有实际用例的规则才沉淀到框架；权限、存在性和业务状态判断留在业务层。
- Controller 有静态请求默认值时，用 `default` tag 和 `github.com/creasty/defaults.Set` 在绑定前预填，再由请求覆盖并校验，检查返回错误。局部更新保留“未传入”和显式零值/空值的语义，不为其隐式补值；完整替换按接口声明处理默认值。业务状态相关默认值仍由 Service 决定。具体例子见 `docs/go-project-layout.md`。
- 日志使用仓库内 `logger` 包，复用 Zap 和 Lumberjack。修改日志时保留请求 ID、级别过滤与文件轮转。许可证中间件依赖 `middleware.LicenseAuthorizer`；客户端创建、启动和关闭由业务项目负责。
- 数据库仅使用公开驱动。新增驱动同时更新配置检查、数据库装配、类型映射和相关测试。
- 幂等仅在鉴权和本次授权之后启用；`middleware.Idempotent` 作为需要幂等的路由的最后一个 handler。控制器归业务服务所有，不在路由重建时新建。

## 修改与验证

- 根模块与四个带 `go.mod` 的示例是独立模块。修改模块路径或依赖时，同步检查示例的 `require`、本地 `replace`、Makefile 的链接变量和 `scripts/gen_code.py`。
- 代码生成器以 `examples/example-hello-world` 为模板。修改模板或生成器后，在临时目录生成独立项目，执行 `go mod tidy` 并构建服务端与 CLI。
- gorgeous 从 `https://github.com/FlyTOmeLight/go_fmt` 的固定版本源码安装，版本与 CI 一致，安装方法见 README。Go 代码修改后执行 `gorgeous ./...`，并用 `gorgeous -d ./...` 检查格式，以符合推送钩子的要求。在受影响模块执行 `go test ./...`、`go vet ./...`。根模块测试需要 `CGO_ENABLED=1` 以使用 SQLite；共享状态和生命周期改动对受影响包执行 `go test -race`。
- 每次框架修改后执行根目录 `make check`，验证根模块、四个独立模块、全部可运行示例与生成项目；必须包含实际启动、HTTP/SDK/CLI 请求和退出，编译成功不足以交付。新示例同步接入 `scripts/check.py` 和 CI。PostgreSQL 检查使用脚本拥有的隔离 Docker 数据库，缺少 Docker 或数据库启动失败应明确失败。
- 发布文件只包含公开依赖、示例值和环境变量占位符。代码索引、运行日志、数据库和本地构建产物由 `.gitignore` 排除。

## 公开版本边界

影响调用方的修复或功能合入 `main` 后发布版本，让消费方可通过版本号取得改动；纯文档和测试改动可随下一版发布。兼容修复递增补丁号，新增能力递增次版本；v0.x 的不兼容接口调整递增次版本，并提供迁移说明。发布标签指向通过完整检查的 `main` 提交，不重写既有标签；同步示例 `require`、生成器和使用说明的正式版本，并验证发布依赖的独立构建。

公开版本从清理后的代码快照建立历史。旧远端与旧 Git 历史保留在本地，不推送到公开仓库。旧版本中的私有 `logger-go` 已替换为本仓库 `logger` 包；私有许可证 SDK 改为调用方注入；依赖私有驱动的达梦数据库支持已移除。
