# Go 服务项目组织约定

新 Go 服务默认使用 server-go。目录遵循 Go 官方的 `cmd` 与 `internal` 建议，框架调用遵循 server-go 的实际接口。结合 agentic-kit 的 Controller、Service、Table、SDK 分工，按业务领域组织代码。现有项目以自己的契约为准。

## 推荐目录

以下是需要 HTTP、数据库和客户端的服务布局。只创建当前需求涉及的目录。

```text
project/
├── go.mod
├── cmd/
│   ├── server/main.go
│   └── cli/main.go                 # 有配套 CLI 时创建
├── internal/
│   ├── service/                    # server-go 生命周期适配与装配
│   ├── app/
│   │   └── order/
│   │       ├── controller.go       # HTTP 绑定、响应、Swagger
│   │       └── service.go          # 业务规则、事务、跨模块编排
│   ├── table/                      # 持久化模型与查询
│   ├── client/                     # 外部协议、错误与客户端生命周期
│   ├── config/                     # 本项目的类型化配置和校验
│   └── workflow/                   # 有复杂运行时才创建相应领域包
├── sdk/go/orderv1/                 # 确有对外 Go SDK 时创建
├── conf/server/config.toml
├── doc/openapi/                    # 有 OpenAPI 时创建
├── deploy/                        # 有部署交付需求时创建
└── Makefile                       # 有重复构建命令时创建
```

`internal/service` 负责框架生命周期适配；`internal/app/order/service.go` 负责订单业务。二者承担不同职责。server-go 不要求业务服务放在仓库根目录的 `service/`。HelloWorld 模板采用 `internal/service`。`example-service1`、`example-service2` 的 `service` 包被组合示例实际导入，因此保留为公开包。agentic-kit 的已有布局按其项目契约维护。

小服务可以先用 `cmd/server` 与一个领域包。复杂运行时应像 agentic-kit 的 agent/runtime、workflow 一样按实际领域拆包。`internal/pkg` 只是目录习惯；新包优先使用明确的领域名称，避免建立通用工具集合。

## 职责与依赖方向

| 位置 | 负责 | 依赖约束 |
| --- | --- | --- |
| `cmd`、`internal/service` | 配置校验、构造依赖、启动与退出 | 显式装配业务对象和基础设施 |
| Controller | 路由、参数绑定、HTTP 响应、Swagger | 调用业务逻辑，不直接编排数据库写入 |
| 业务 Service | 业务默认值、权限规则、事务、跨领域调用 | 使用实际需要的存储和外部客户端 |
| Table | 持久化模型、过滤条件、查询和更新 | 接收 context 与数据库句柄，不处理 HTTP 响应和用户权限 |
| Client | 外部系统的协议与错误转换、连接所有权 | 接收配置，显式构造；请求接收 context |
| SDK | 对外请求、响应、客户端契约 | 不导入服务端 `internal` 实现 |

这是职责划分，不是要求每层都建立 Go interface。Controller 和业务 Service 可以先在同一个领域包中；存储只有一个明确实现时也可以直接使用具体类型。需要隔离外部协议或支持实际替换时，在使用方定义小接口。

SDK 中的请求和响应是接口契约，可以在 Controller 与业务 Service 中复用。持久化模型与运行时对象按自己的职责定义，通过明确映射转换。无需为每一层复制同一组 DTO（用于传输的数据结构），也不把数据库字段直接变成接口契约。

事务由业务 Service 定义范围，Table 使用传入的事务句柄。业务默认值由业务 Service 填充。列表筛选、分页、排序尽量在数据库完成；排序需稳定。更新需要区分“未传入”和“清空”时，使用指针或明确的更新类型。时间默认按 UTC 存储和返回。

## 与当前 server-go 接口结合

1. 业务项目有自己的 module path，引用已发布的 `github.com/FlyTOmeLight/server-go`。本地联合开发才使用临时 `replace` 或 `go.work`。
2. `conf/server/config.toml` 是框架当前默认路径。`SERVER_CONFIG_PATH` 可指定其他文件。Go 官方目录建议没有规定 TOML 文件必须放在哪里。业务配置类型放在 `internal/config`，配置文件不与 Go 配置加载代码混放。
3. `cmd/server` 使用 `server.NewBootstrap` 装配真正需要的 DB、Redis 等模块。可失败的依赖构造先完成，再创建业务服务；初始化成功后安排退出清理。
4. 业务服务实现 `host.Service`，通过 `host.NewHTTPHost(manager, log, setup, services...)` 显式装配。`RegisterRoutes(*gin.RouterGroup) error` 只安装路由；可失败的连接和迁移在构造函数或资源模块中完成。
5. `Start(ctx) error` 启动所属后台任务；失败时清理本次部分启动的资源。`Stop(ctx) error` 取消并等待任务退出。`Run(ctx)` 先排空 HTTP，再取消 worker 并逆序停止服务。进程入口负责信号与资源模块关闭。路由动态更新只重建 handler，不重新启动 worker。
6. HTTP 入口向业务逻辑传递 `c.Request.Context()`。业务与 Table 使用 `context.Context` 作为第一个参数，GORM 调用 `WithContext(ctx)`，Resty 请求调用 `SetContext(ctx)`。不要只把 `*gin.Context` 存进共享客户端。
7. 配置、DB、Redis、MongoDB、幂等和订阅集合均由实例拥有。配置使用 `manager.Snapshot().UnmarshalKey`；DB 和客户端显式传给业务对象。可选 HTTP 集成通过 `httpintegration.Install` 或自己的 setup 安装。配置动态更新范围见 [conf/README.md](../conf/README.md)。
8. 后台任务持有明确的取消函数和等待手段。只有确实需要独立部署时才增加 `cmd/worker`；agentic-kit 的独立 automation worker 目前是受 build tag 控制的入口，不应复制成所有新项目的必备目录。

## 不机械继承的约定

agentic-kit 的 RPC 路由风格、业务枚举、数据库字段和外部系统是它自己的契约。它的分层职责、context 传递和 worker 退出管理可以复用。包级默认客户端、庞大的统一装配文件和 `internal/pkg` 集合不作为新项目的默认设计。

框架仓库本身属于公共库，需要保留供调用方导入的领域包；业务服务通常是独立程序，私有实现放 `internal`。公共 SDK 或其他包实际需要单独分发时，再考虑独立模块。不要为一个业务服务提前创建多个 go.mod。

## 完成判据

新服务应能从 `cmd/server` 启动，配置路径正确，业务请求完成端到端流程，收到退出信号后释放所属资源。目录调整必须同步 import、生成器、Swagger 扫描目录、配置路径和构建脚本。Go 代码完成后执行相关测试与 `go vet`；SQLite 构建与运行必须启用 CGO。

## 依据

- [Go 官方模块布局建议](https://go.dev/doc/modules/layout)：服务入口集中在 `cmd`，服务实现优先放 `internal`；公共库和服务程序的组织方式不同。
- [Go 官方包命名建议](https://go.dev/blog/package-names)：包名表达具体职责，避免通用工具包和集中所有接口的包。
- [Go context 文档](https://pkg.go.dev/context)：沿调用链显式传递 context，通常不把它存入结构体。
- agentic-kit 的 `docs/coding_spec.md`、`AGENTS.md`、`cmd/server/server.go`、`service/service.go`、`internal/table/agent_run.go`：提炼分层、接口契约同步、事务和取消传播规则。
- server-go 的 `bootstrap.go`、`host/host_default.go`、`conf/conf.go` 及 HelloWorld 示例：确定框架当前要求，区分已有接口与未来改进建议。
