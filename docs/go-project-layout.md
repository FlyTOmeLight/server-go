# Go 服务项目组织约定

新 Go 服务默认使用 server-go。目录遵循 Go 官方的 `cmd` 与 `internal` 建议，框架调用遵循 server-go 的实际接口。按业务领域组织代码，明确 Controller、Service、Table、SDK 的职责。现有项目以自己的契约为准。

## 推荐目录

以下是需要 HTTP、数据库和客户端的服务布局。只创建当前需求涉及的目录。

```text
project/
├── go.mod
├── cmd/
│   ├── server/server.go
│   └── cli/cli.go                 # 有配套 CLI 时创建
├── service/
│   ├── build.go                    # 类型化 Config 与独立应用装配
│   └── service.go                  # 公开服务构造与生命周期
├── internal/
│   ├── app/
│   │   └── order/
│   │       ├── controller.go       # HTTP 绑定、响应、Swagger
│   │       └── service.go          # 业务规则、事务、跨模块编排
│   ├── table/                      # 持久化模型与查询
│   ├── client/                     # 外部协议、错误与客户端生命周期
│   ├── config/                     # 本项目的类型化配置和校验
│   └── workflow/                   # 有复杂运行时才创建相应领域包
├── command/                        # 有 CLI 时组织领域命令
├── sdk/go/orderv1/                 # 确有对外 Go SDK 时创建
├── conf/server/config.toml
├── doc/openapi/                    # 有 OpenAPI 时创建
├── deploy/                        # 有部署交付需求时创建
└── Makefile                       # 有重复构建命令时创建
```

`service/` 与 `cmd/` 同级，公开服务构造与 `host.Service` 实现，供独立入口和组合服务导入；`internal/app/order/service.go` 负责私有订单业务。公开服务包可以访问自身模块的 `internal` 实现，组合项目只依赖公开构造函数。HelloWorld、example-service1 和 example-service2 都采用这种组织。

所有 HTTP 示例以 HelloWorld 为基准：入口只处理信号与退出码，`service.Build` 通过 `app.Runtime` 接管实际需要的资源，再交付服务。组合应用自行声明整体 `Config`，通过公开服务构造函数注入共享资源；不调用各子服务的独立 `Build` 重复创建连接。迁移开关显式传入数据库服务的构造函数。

无状态小服务采用 `cmd/server`、`service` 和一个 Controller 即可，无需空业务 Service 或 Table。独立 worker 使用 `cmd/worker`、公开 `service` 与私有任务包，继续使用标准 context；没有 HTTP 时不创建 HTTPHost、HTTP 配置或空路由。全部可运行例子见 [examples/README.md](../examples/README.md)。

小服务可以先用 `cmd/server` 与一个领域包。复杂运行时按实际领域拆包，例如任务执行或工作流编排。`internal/pkg` 只是目录习惯；新包优先使用明确的领域名称，避免建立通用工具集合。

## 职责与依赖方向

| 位置 | 负责 | 依赖约束 |
| --- | --- | --- |
| `cmd`、`service` | 配置校验、构造依赖、启动与退出 | 显式装配业务对象和基础设施 |
| Controller | 路由、参数绑定、HTTP 响应、Swagger | 调用业务逻辑，不直接编排数据库写入 |
| 业务 Service | 业务默认值、权限规则、事务、跨领域调用 | 使用实际需要的存储和外部客户端 |
| Table | 持久化模型、过滤条件、查询和更新 | 接收 context 与数据库句柄，不处理 HTTP 响应和用户权限 |
| Client | 外部系统的协议与错误转换、连接所有权 | 接收配置，显式构造；请求接收 context |
| SDK | 对外请求、响应、客户端契约 | 不导入服务端 `internal` 实现 |

这是职责划分，不是要求每层都建立 Go interface。Controller 和业务 Service 可以先在同一个领域包中；存储只有一个明确实现时也可以直接使用具体类型。需要隔离外部协议或支持实际替换时，在使用方定义小接口。

SDK 中的请求和响应是接口契约，可以在 Controller 与业务 Service 中复用。持久化模型与运行时对象按自己的职责定义，通过明确映射转换。无需为每一层复制同一组 DTO（用于传输的数据结构），也不把数据库字段直接变成接口契约。

事务由业务 Service 定义范围，Table 使用传入的事务句柄。业务默认值由业务 Service 填充。列表筛选、分页、排序尽量在数据库完成；排序需稳定。更新需要区分“未传入”和“清空”时，使用指针或明确的更新类型。时间默认按 UTC 存储和返回。

## Go 服务端 SDK 约定

新建、修改或审查服务端项目的 Go SDK 时采用本节。按领域拆分接口，复用契约类型并验证实际请求行为；已有协议按项目契约维护，客户端构造与请求生命周期使用当前 server-go 接口。

### 文件与契约

SDK 放在 `sdk/go/<服务名或领域名>v1`，按业务领域拆请求/响应和调用方法，避免把全部接口放进一个文件。文件组织示例：

```text
sdk/go/orderv1/
├── order_client.go             # Client、配置与构造函数
├── order_api_order.go          # 订单请求、响应、枚举与公开数据类型
├── order_client_order.go       # 订单 HTTP 调用
└── order_client_order_test.go  # 请求与响应契约测试
```

每个操作定义自己的 `CreateOrderRequest`、`UpdateOrderRequest` 等请求类型，便于分别演进校验与字段语义。共用的领域数据和分页字段按实际语义复用；响应完全相同时可以复用同一类型，无返回数据的方法直接返回 `error`。Controller 与业务 Service 复用这些契约，持久化模型通过明确映射转为响应。SDK 不依赖服务端 `internal`、GORM 模型或 Gin context。

新 JSON 字段默认用小驼峰，Go 字段保留规范的缩写，如 `ProjectID` 对应 `projectID`。请求包含明确类型、`json` 标签和必要的 `binding`、`example`；响应不加请求校验标签。枚举使用字符串类型及具名常量，值按项目协议确定。更新字段用指针或明确的更新类型区分未传入与清空；嵌套字段的省略、`null`、空对象以及列表的空数组均按实际契约确定，不能统一加 `omitempty`。

请求的必填、长度、数值范围、枚举和格式约束放在 SDK 请求类型的 `binding` tag 中，优先采用 validator 内置的 `required`、`max`、`min`、`oneof`、`uuid` 等规则。Query 参数声明 `form` tag，路径参数声明 `uri` tag；不属于请求体的字段使用 `json:"-"`。Controller 通过 `binder.Bind(ctx, &req).All().Error()` 统一处理绑定与校验错误，完整合并请求来源后再校验。跨项目通用规则复用 `pkg/binder`；只有真实用例不能用现有规则表达时再添加。资源是否存在、调用方权限和业务状态仍在 Service 判断。

静态请求默认值可使用 `default` tag。Controller 在绑定前调用 `github.com/creasty/defaults.Set`，再由用户输入覆盖默认值并通过 binder 校验：

```go
var req sdk.ListOrderRequest
if err := defaults.Set(&req); err != nil {
    _ = ctx.Error(err)
    return
}
if err := binder.Bind(ctx, &req).All().Error(); err != nil {
    _ = ctx.Error(err)
    return
}
```

这样用户明确传入的 `0`、`false` 或 `null` 不会在绑定后被再次补值。需要区分未传入和空值时仍使用指针或明确的更新类型；局部更新不声明会改变未传入字段的默认值。完整替换接口可按已声明的契约使用默认值，例如 PostgreSQL 示例的 PUT。涉及用户、资源状态或业务规则的默认值由 Service 决定。库行为见 [defaults 文档](https://pkg.go.dev/github.com/creasty/defaults@v1.11.0)。

SDK 方法的路径、HTTP 方法、请求编码、响应结构与 Controller、Swagger 一致。路由风格遵循项目协议。分页沿用接口的页码/页大小或游标方式；有总数契约时返回总数与当前页结果，不把分页查询隐式变成全量拉取。

### 客户端与请求

- 显式构造 Client 并返回 `(*Client, error)`，校验 endpoint 与支持的鉴权配置，复用框架 `sdk.Client` 的连接、超时与传输能力。所有者在调用结束后关闭客户端；业务 SDK 不新增包级默认实例。消费方需要替换实现时，在消费方定义实际使用的小接口，不因接口数量增加而生成一个必须实现全部方法的大接口。
- 新方法使用 `CreateOrder(ctx context.Context, req *CreateOrderRequest) (*CreateOrderResponse, error)` 这样的签名。每次创建新的 HTTP 请求，并用 `SetContext(ctx)` 传递取消和截止时间。入站身份、幂等 token、请求 ID 等只写入本次请求；共享 Client 不保存入站 context，不在每次调用时修改默认 Header 或 Transport，也不通过 Clone 传播身份。内部 SDK 适配在 HTTP 边界提取身份和请求 context，公共 SDK 保持与 Gin 无关。
- v0.8.0 起，普通 JSON 方法在 Resty 完成请求与结果解码后调用 `sdk.Complete(result, err)`，集中处理传输错误和 HTTP 失败；SSE 保持独立的流读取与关闭流程。正式消费需使用 v0.8.0 或更高版本。普通 JSON 调用保留网络错误及 context 取消的错误身份；HTTP 失败解析为可检查的服务错误，并保留实际状态码和服务错误码。错误响应不是预期 JSON 时，仍返回带状态码的错误，不能把空错误结构或默认响应当成功。成功只返回本次解析出的业务数据，不向调用方暴露 Resty Response；SDK 不对每次失败再打印日志或自行退出进程。
- 普通请求使用明确的超时。只有操作契约允许安全重试时才启用重试；创建、执行等有副作用的操作需要服务端幂等保障，不能仅因为采用 POST 就统一重试。流式调用应单独配置时限，保留调用方取消，不将 `SetTimeout(0)` 作为所有方法的默认值。
- 流式接口直接返回流或统一的流式响应，例如包含 `io.ReadCloser`、状态码和 Header 的类型，不预先把完整响应读进内存。流式 HTTP 失败按契约转换为错误，或在明确要求透传时返回状态与响应体。返回响应后由调用方关闭流；返回错误前由 SDK 关闭自己持有的响应体。SSE（服务端持续发送事件的协议）的事件格式按接口契约处理。

### SDK 变更验证

变更同步请求/响应类型、客户端方法、Controller 路由、Swagger 与相关调用方；需要重新生成 OpenAPI 时执行项目已有文档命令并检查产物。用 `httptest.Server` 或已有 Transport 测试真实请求路径、HTTP 方法、JSON 编码和响应解析。按变更涉及的风险覆盖服务错误、非 JSON 错误、取消/超时，以及流的读取和关闭；修改共享请求状态时运行竞态检测。服务端已有实现应通过 SDK 完成相关端到端调用，不仅检查接口方法能否编译。

## CLI 执行与迁移（v0.8.0）

`CommandLine` 实例拥有自己的配置、Apply 注册、SDK 客户端和日志。`SetConfigFile(path)` 只选择该实例的配置文件；默认文件路径、环境前缀和 `ConfigFilePath` 构建链接变量沿用原约定。构造命令不会读取或创建配置文件，也不会启动配置监听；`config set/get/unset` 执行时读取文件，缺少文件时创建，业务命令执行时检查配置。

命令通过 `cli.Client(cmd.Context())` 获取当前执行的 SDK 客户端，并把 `cmd.Context()` 传给 SDK。领域 Apply 在 `NewCommand()` 中调用 `cli.RegisterCommandApplyHandler(cmd, kind, input, func(ctx context.Context, args any) error)`，注册随 `Append` 归属当前实例，回调传递收到的 context。示例保持 `CommandBuilder.NewCommand()` 和 `Append` 的既有调用形状。

嵌入式调用和测试使用 `commandline.ExecuteContext(ctx)`，它返回错误并在成功、失败或取消后关闭客户端和日志；再次执行使用新的执行 context。进程入口使用 `commandline.Execute()`，它处理中断信号、SIGTERM 和退出码。不要直接调用 `commandline.Root.Execute()` 绕过执行资源清理。

从旧 CLI 迁移时，将 `cli.Config.Client` 替换为 `cli.Client(cmd.Context())`，去掉领域包级客户端，将 `RegisterApplyHandler` 的回调改为 `RegisterCommandApplyHandler` 的 context 回调。`CommandLine` 不再更新全局 Viper 或包级 `Config`。旧 `SetConfig`、`Config`、`GetApplyHandler` 和 `RegisterApplyHandler` 保留给明确管理全局状态的手动调用；手动创建的客户端由调用方关闭。旧 Apply 注册在命令行构造或旧 builder 注册时复制为实例快照，后续全局注册不会改变已装配实例。

本节约定从 v0.8.0 提供。升级旧项目时，按上述步骤迁移命令的客户端获取和 Apply 注册；v0.7.0 及更早版本仍遵循其既有 CLI 契约。

## 与当前 server-go 接口结合

1. 业务项目有自己的 module path，引用已发布的 `github.com/FlyTOmeLight/server-go`。本地联合开发才使用临时 `replace` 或 `go.work`。
2. `conf/server/config.toml` 是框架当前默认路径。`SERVER_CONFIG_PATH` 可指定其他文件。Go 官方目录建议没有规定 TOML 文件必须放在哪里。装配所需的类型化配置放在 `service/build.go`；配置复杂且有独立业务校验时才拆到 `internal/config`。配置文件不与 Go 配置加载代码混放。
3. 当前工作树使用 `app.Run(ctx, options, build)` 编排配置、日志、资源、HTTP 和退出清理。业务声明配置结构体，由 `app.Run` 按字段名匹配配置 key，名称不同时用 `mapstructure` tag 映射，一次解析后传给装配函数；业务直接读取类型化字段。装配函数通过 `runtime.Initialize(ctx, modules...)` 初始化真正需要的 DB、Redis 等模块，再显式构造服务并通过 `runtime.AddService` 交付。构造失败前尚未交付的部分资源由构造函数清理。该入口从 v0.5.0 开始提供；消费更早版本时仍使用 `server.NewBootstrap` 和 `host.NewHTTPHost` 显式装配。
4. 业务服务实现 `host.Service`，通过 `host.NewHTTPHost(manager, log, setup, services...)` 显式装配。`RegisterRoutes(*core.RouterGroup) error` 只安装路由；可失败的连接和迁移在构造函数或资源模块中完成。
5. `Start(ctx) error` 启动所属后台任务，启动期间响应取消；失败时清理本次部分启动的资源。装配与服务启动共享 `app.Options.InitTimeout` 预算。`Stop(ctx) error` 取消并等待任务退出，能处理部分启动并允许失败后重试；返回 nil 才确认依赖可以关闭。`Run(ctx)` 先排空 HTTP，再取消 worker 并逆序停止服务，停止超时后继续等待退出；停止仍失败时保留依赖并返回错误。进程入口负责信号和退出码，`app.Run` 负责已接管资源的关闭。路由动态更新只重建 handler，不重新启动 worker。
6. HTTP Controller 和对应业务 Service 使用 `*core.Context`，便于读取身份和请求信息。数据库及网络操作使用 `ctx.Request.Context()` 获取的标准 context，传给 GORM 的 `WithContext` 和 SDK 的 context 参数；SDK 与长期任务仍使用 `context.Context`。共享客户端和数据库后台操作不持有会被 Gin 复用的 Context。
7. 配置、DB、Redis、MongoDB、幂等和订阅集合均由实例拥有。配置由 `app.Run` 一次解析为业务结构体，不在各模块重复调用 `Snapshot().UnmarshalKey`；同名字段无需 `mapstructure` tag。DB 和客户端显式传给业务对象。可选 HTTP 集成通过 `httpintegration.Install` 或自己的 setup 安装。配置动态更新范围见 [conf/README.md](../conf/README.md)。
8. 后台任务持有明确的取消函数和等待手段。只有确实需要独立部署时才增加 `cmd/worker`，不把独立 worker 入口作为所有新项目的必备目录。

## 通用 HTTP 能力归属

HTTPHost 负责请求 ID、请求日志、Recovery、404/405 和标准错误响应。业务 Controller 复用 `pkg/binder` 的严格 JSON 绑定和分页参数解析，通过 `errors.AbortWithError` 返回框架错误；业务只保留领域错误与 HTTP 状态的映射。静态服务凭据复用 `auth.ParseBearerTokens`、`auth.BearerTokens` 和 `auth.UserID`，项目权限与资源归属校验留在业务层。无需在每个服务复制 HTTP 工具包或相同中间件。

SDK 使用 `sdk.DecodeError` 解析框架错误，向调用方保留 `errors.Error`；`Status` 表示 HTTP 状态，`StatusCode` 表示框架错误编号，不能混用。新增能力的发布边界见 README。

## 项目契约与公共边界

路由风格、业务枚举、数据库字段和外部系统由项目契约决定。分层职责、context 传递和 worker 退出管理遵循本约定。新项目使用显式依赖注入，按领域分配装配职责和共享代码归属。

框架仓库本身属于公共库，需要保留供调用方导入的领域包；业务服务的可组合入口放公开 `service/`，私有领域实现放 `internal`。公共 SDK 或其他包实际需要单独分发时，再考虑独立模块。不要为一个业务服务提前创建多个 go.mod。

## 完成判据

新服务应能从 `cmd/server` 启动，配置路径正确，业务请求完成端到端流程，收到退出信号后释放所属资源。目录调整必须同步 import、生成器、Swagger 扫描目录、配置路径和构建脚本。Go 代码完成后执行相关测试与 `go vet`；SQLite 构建与运行必须启用 CGO。

## 依据

- [Go 官方模块布局建议](https://go.dev/doc/modules/layout)：服务入口集中在 `cmd`，服务实现优先放 `internal`；公共库和服务程序的组织方式不同。
- [Go 官方包命名建议](https://go.dev/blog/package-names)：包名表达具体职责，避免通用工具包和集中所有接口的包。
- [Go context 文档](https://pkg.go.dev/context)：沿调用链显式传递 context，通常不把它存入结构体。
- server-go 的 `bootstrap.go`、`host/host_default.go`、`conf/conf.go` 及 HelloWorld 示例：确定框架当前要求，区分已有接口与未来改进建议。
