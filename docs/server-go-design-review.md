# server-go 设计审查

审查基准：`19f342dfe87d5dcb1765890bd5287a27f809f186`（公开版本 `v0.1.0`）。范围包括启动、配置、HTTP、中间件、SDK、资源生命周期和项目生成。下列 F/D 条目保留原始问题与基准版本行号；修复结果对应 `v0.2.0`，当前实现以源码为准。

原审查结论：主要问题集中在状态与资源所有权。包级默认对象、配置回调和服务启动相互依赖，但接口没有完整表达失败、取消、替换和关闭。修复将配置与资源归应用实例所有，并补齐失败、取消和关闭路径。

P1 表示可能造成数据泄漏、数据丢失、重复业务操作或核心启动/运行失败，需要优先修复。P2 表示明确的功能缺陷或影响可维护性的接口设计。

## 可复现问题

### F1 · P1 · 幂等缓存命中绕过后续鉴权

位置：`middleware/idempotence.go:37`、`idempotence/idempotence.go:50`、`host/host_default.go:114`。

缓存仅按 `clientToken` 索引，比较指纹只包含 URL 和 body。缓存命中直接返回响应并中止后续中间件。HTTPHost 在幂等中间件之后安装鉴权；业务在 `Initialize` 注册的鉴权也位于其后。

复现：Alice 的合法 POST 写入缓存；Bob 使用相同 URL、body 和 token，再请求同一路由。后置鉴权本应拒绝 Bob，实际返回 `200 {"owner":"alice"}`。身份不在缓存键中，缓存命中时也不再执行鉴权。

建议：先完成身份验证与本次授权，再在已验证身份范围内使用幂等缓存。缓存键表达租户、主体、操作和 token。缓存命中仍须满足当前授权条件。

### F2 · P1 · 同一 token 的并发请求仍执行两次

位置：`middleware/idempotence.go:37`、`:60`、`:69`。

读取缓存与写入结果之间没有原子占用。两个并发请求在第一个响应完成前都看到空缓存，都会执行业务操作。

复现：让两个同 token 请求同时进入 handler，在双方都到达后放行。handler 实际执行两次。

建议：在执行操作前原子声明 token 的执行状态；后续同指纹请求等待同一次执行的结果。明确当前只在单进程内保证幂等；需要跨副本保证时再采用能够原子占用的共享存储。

### F3 · P1 · HTTP 热重载阻塞配置监听回调

位置：`host/host_default.go:250`、`:275`、`conf/conf.go:198`。

重载回调关闭旧 HTTP Server，创建新 Server 后调用 `h.Run()`。Run 等待退出信号，因此回调一直不返回。配置发布位于回调之后，无法及时完成。新 Run 还使用 `context.Background()`，脱离原 RunWithContext 的生命周期，并重复调用服务 StartAsync。

复现：改变 HTTPServer.ReadTimeout，调用重载。新服务器启动，但回调在观察期限内持续阻塞。

建议：一个 Host 只拥有一次 Run 生命周期。配置回调向该生命周期提交变更；不要重新进入 Run。明确可动态更新的选项，业务 worker 不随 HTTP Handler 重建而重复启动。

### F4 · P1 · 配置热更新对共享对象原地写入

位置：`conf/conf.go:92`、`:204`、`conf/types.go:93`、`conf/conf.go:52`。

Default 返回可变的包级指针。SetDefault 和配置监听会原地修改该对象，HTTP 处理和其他调用方直接读取，没有统一同步机制。

复现：并发 SetDefault 与读取 Default().AppName，Go 竞态检测报告 Config.reset 写入与读取竞争。增加普通读锁无法保护已交给调用方的可变指针。

建议：配置加载、校验、发布由一个实例负责。发布不可变快照；读取方获取稳定版本。回调和资源替换应关联同一配置版本，并在失败时保留上一个有效版本。

### F5 · P1 · 生成器无条件删除已有项目

位置：`scripts/gen_code.py:153`、`:174`。

生成目标存在时直接 shutil.rmtree，再用生成结果替换。没有要求覆盖的参数，也不区分已有源码与上次生成物。同名 `_temp` 目录也会直接删除。

复现：在目标项目中创建 existing-work.txt，再用同一名称运行生成器。命令退出码为 0，已有文件消失。

建议：目标存在时拒绝生成。临时目录使用系统唯一目录。若以后明确需要覆盖，单独定义可覆盖范围；不能把整个项目目录视为可删除的生成物。

### F6 · P1 · 默认配置加密使用公开固定密钥

位置：`conf/decode.go:24`、`:150`。

未设置 SERVER_SECRET_KEY 和 SERVER_SECRET_IV 时，EncryptSecret 与解密器使用源码中的固定值。持有密文的人可以按公开实现解密；同一明文重复加密还产生相同密文。

建议：缺少密钥时明确返回错误。需要配置密文时使用认证加密，例如标准库 AES-GCM，随机 nonce 与密文一起保存；或直接使用现有环境变量、文件注入机制。格式变更需显式定义，不静默降级。认证加密的能力参见 [Go cipher 文档](https://pkg.go.dev/crypto/cipher#AEAD)。

### F7 · P1 · HelloWorld 默认构建与 SQLite 配置矛盾

位置：`examples/example-hello-world/Makefile:71`、`examples/example-hello-world/conf/server/config.toml:58`。

示例默认数据库为 SQLite，Makefile 却用 CGO_ENABLED=0 构建服务端。所用 go-sqlite3 驱动需要 CGO。

复现：按同一选项构建成功，但运行二进制立即退出 1，错误包含 `go-sqlite3 requires cgo to work. This is a stub`。生成器复制这个 Makefile，因此新项目也继承该问题。

建议：SQLite 服务端构建启用 CGO；配套 CLI 可独立设置构建选项。部署目标需要相应 C 工具链。构建验证必须包含实际启动，不能以编译成功替代运行验证。

### F8 · P2 · Bootstrap 没有初始化回滚与依赖逆序清理

位置：`bootstrap.go:37`、`:53`。

后一个模块初始化失败时，之前已初始化的模块没有回滚。Close 按初始化顺序关闭所有模块，也不记录哪些模块已成功。依赖模块可能在使用的资源已经关闭后才执行清理；未初始化的 db.Module.Close 还会访问空 DB。

复现：模块 A 初始化成功，B 返回错误；A 未关闭。随后 Close 的顺序为 A、B。

建议：Bootstrap 维护成功初始化的模块序列，失败时逆序回滚，正常退出也逆序关闭。关闭应能报告错误，并按实际资源需要接受带截止时间的 context。

### F9 · P2 · 初始化失败后第二次调用返回成功

位置：`conf/conf.go:33`。

错误保存在调用内的局部变量中，sync.Once 无论初始化成功与否都会完成。第二次调用不执行初始化，局部 err 是 nil，Default 却仍可能为空。

复现：配置文件不存在；第一次 Initialize 返回错误，第二次返回 nil，defaultConfig 仍为 nil。

建议：显式构造配置实例并返回其初始化结果。若确有进程单次初始化契约，至少持久保存失败结果，不能向调用方报告成功。

### F10 · P2 · 合并式加载保留已删除配置

位置：`conf/conf.go:100`、`:191`。

NewConfig 使用全局 Viper.MergeInConfig。新文件中删除的键仍保留在已有状态中；changedKeys 只遍历旧 AllSettings，新增顶层键也不会被完整识别。

复现：首次文件含 Feature.Old=true；删除该节后重新加载，Feature.Old 仍为 true。

建议：每次使用独立加载器解析完整文件，再与旧快照比较键的并集。成功后发布新快照，而不是持续合并进同一个全局容器。

### F11 · P2 · SDK 克隆与包装丢失调用语义

位置：`sdk/client.go:44`、`internalsdk/client.go:90`、`sdk/client.go:74`。

Clone 根据 Config 重新创建 Resty Client，只复制 transport；调用方添加的签名中间件、默认 header 等配置没有保留。内部 Wrap 只读取 Gin Context 中的身份，没有把 Request.Context 设置给下游请求。AK/SK 和 STS 配置还暴露为可选能力，但实现分别是 TODO。

复现一：原客户端有签名中间件，克隆后请求返回 401。复现二：包装已取消的入站请求，再发起下游调用，仍执行一次并返回 200。

建议：客户端持有连接与稳定设置；每次调用接受 context，并在请求级添加身份信息，避免为身份传播重新构造整个客户端。支持的鉴权模式必须完整实现；不支持的模式在构造时返回明确错误。公开嵌入整个 Resty Client 会扩大配置契约，必须与 Clone 的承诺一致。

### F12 · P2 · 畸形配置密文触发 panic

位置：`pkg/safety/aes_cbc.go:47`、`:51`、`:74`。

解密前不校验 IV、密文分块长度或填充。空密文会访问 text[-1]，非完整块会由 CryptBlocks 触发 panic。配置加载调用的是返回 error 的函数，却无法按错误路径处理这些输入。

复现：CBCDecrypter 接收空字符串或 `YQ==`，分别出现数组越界与 `input not full blocks` panic。

建议：复用标准认证加密，并把无效数据作为 error 返回。修复现有格式时至少完成输入长度和填充检查，避免依赖 recover 掩盖解析错误。

### F13 · P2 · 定时任务把 Duration 的纳秒值当成秒步长

位置：`cron/period_task.go:211`。

fmt.Sprintf 使用 time.Duration 的底层整数构造 cron 秒字段。传入 5*time.Second 后得到步长 5000000000，而不是 5。

复现：计算下一次调度时间，预期 5 秒，实际为 1 分钟。

建议：固定间隔使用调度库的 duration 接口；若保留 cron 表达式，应先明确单位和有效范围。任务管理器退出还应等待正在执行的任务，并关闭任务自身资源。

## 结构性问题

### D1 · 配置、运行状态与基础设施交叉依赖

conf 依赖 sdk、internalsdk，internalsdk 又依赖通用 pkg。`go list -deps ./conf` 确认仅配置包就间接依赖 Gin 与 Resty。配置模块既解析配置，又构造领域客户端配置。

db、redis、conf、idempotence、pubsub 均维护包级状态。例如第二个 DB Module 会覆盖第一个默认数据库；Dapr 订阅集合也不属于某个 Host。多 Host、多个数据库或多个测试实例无法自然隔离。

建议让配置解析只产出类型化数据；客户端转换留在装配处。资源由应用实例持有，业务通过构造函数拿到实际依赖。

### D2 · 生命周期接口没有表达失败、取消与完成

host.Service.Initialize 无 error，StartAsync/StopAsync 无 context 或完成结果。AddService 接收 LifeCycle，却只保留同时实现 Service 的对象，其他对象会被静默忽略。

RunWithContext 在处理退出时先 StopAsync，再 Shutdown；HTTP 仍在处理的请求可能使用已经停止的业务资源。redis.Module.Close 与 Reload 是空实现，实际 Redis Client 的关闭不在 Bootstrap 清理链中。TTL Store 的清理 goroutine 也没有退出路径。

建议定义一条完整启动与退出链：构造依赖 → 安装路由 → 启动 HTTP/worker → 停止接收并排空 HTTP → 取消并等待 worker → 逆序关闭资源。扩展接口只服务这些实际需要。

### D3 · 导入通用包就改变进程环境

pkg/log.go 的 init 创建日志对象和目录，读取当前工作目录配置，设置默认日志器，并可能 panic。仅导入 pkg.Context、KVStore 或 ID 工具，也会启动日志配置。库调用方无法在 main 中处理这段初始化失败。

上一轮公开迁移替换了私有日志实现，但仍保留了这种隐式初始化，并新增了构造失败时的 panic。它应作为本框架的设计问题一起修正。

建议显式构造日志并注入 HTTP/业务模块；由拥有者统一关闭。不要让通用类型包承担日志生命周期。

### D4 · 通用包与集成包没有清楚的职责界线

pkg 混合 Gin 别名、日志、ID、文件、缓存、KV 和格式化；concurrency 包只有一个空白导入，调用方没有可用行为。HTTP Host 又直接接入 Console、Dapr、监控和认证。框架的稳定核心与可选集成难以单独使用和验证。

建议先按已有实际职责划分，不建立新的框架层级：HTTP 生命周期、配置、日志为清楚的模块；集成由装配代码显式安装。纯转发别名和无行为包不构成新的抽象。是否分为独立 Go 模块，取决于实际分发需求和验证后的依赖收益。

## 修复结果（v0.2.0）

| 记录 | 修复与回归证据 |
| --- | --- |
| F1 | 移除 Host 全局幂等；路由在鉴权和本次授权之后显式包装。缓存隔离租户、用户与 HTTP 操作；不同主体和授权撤销有回归测试。 |
| F2 | 先原子占用 token；同指纹并发请求等待同一次结果，不同指纹返回 409。测试覆盖取消等待、状态/正文/Header 重放及失败后重试。 |
| F3 | Host 只有一次 Run；新路由原子替换，不重新启动 worker。静态传输变更明确要求重启。 |
| F4 | 独立 Manager 返回配置副本，替换前完成全部校验，失败保留有效版本；并发读写通过 race。 |
| F5 | 生成器拒绝已有目标，临时目录唯一；回归验证既有源码保留及失败退出码。 |
| F6 | AES-GCM 使用显式密钥与随机 nonce，认证版本前缀；缺密钥、篡改和错误密钥均失败。 |
| F7 | SQLite 服务端启用 CGO；生成项目完成实际启动与 CRUD，CLI 可独立禁用 CGO。 |
| F8 | Bootstrap 记录成功模块，逆序回滚和关闭，聚合错误；初始化取消后使用独立清理期限，失败资源保留供 Close 重试。 |
| F9 | 配置构造每次返回真实结果，无 sync.Once 吞错；重复缺失文件及无效配置测试通过。 |
| F10 | 每次完整解析独立配置；比较新旧顶层键并集。删除、新增与有效默认值有回归测试。 |
| F11 | Resty 原生克隆保留设置；内部请求包装继承入站 context，身份只写入本次请求。不支持的 AK/SK、STS 和内部自动幂等在构造时返回错误。 |
| F12 | 配置解密使用认证格式；CBC 工具校验 IV、块长与完整填充，无效输入返回错误。 |
| F13 | 调度使用 cron.Every(duration)。后台工作 context 归任务生命周期所有；Stop 排空任务并用独立期限清理，关闭失败可重试。 |
| D1 | conf 不再构造 SDK。DB、Redis、MongoDB、幂等与订阅集合为独立实例，基础设施由装配入口构造和注入。 |
| D2 | Module 与 Service 返回 error 并接收 context；HTTP 排空后取消并等待 worker，再关闭资源。Modbus 拥有连接和任务，选举任务在任期结束时取消并等待。 |
| D3 | 删除 pkg 的隐式日志初始化与全局 HTTP/DB Logger。入口显式构造日志，通过请求 context 注入；单纯导入 pkg 不读取配置或创建文件。 |
| D4 | HTTPHost 保留 HTTP 生命周期和基础路由；可选能力显式安装。删除无行为 concurrency 包、未使用 TTL Store 和空模板 Client。Dapr 发布拥有并关闭独立连接。Gin 别名作为框架既有公开契约保留。 |

集成复查额外覆盖：Host 构造与配置替换的原子订阅、路由重建期间 Gin 校验器只注册一次、FakeAuth 默认值只在构造时赋值、HTTP 强制关闭后的请求退出与独立服务清理预算、任务初始化取消后的资源保留与清理重试。

## 接口迁移

本版本调整公开接口。旧 Service 的 Initialize/StartAsync/StopAsync 改为 RegisterRoutes/Start(ctx)/Stop(ctx)。RegisterRoutes 只安装路由，可重复在新 Router 上执行；连接和 worker 不在此处创建。调用方负责处理启动错误、信号和资源关闭。完整装配见最小示例和 HelloWorld 模板。

幂等控制器由业务服务持有，最后的路由 handler 使用 middleware.Idempotent；匿名请求不缓存，只提供单进程保证。配置的监听地址、协议、超时、Header 限制与基础设施变更需要进程重启，动态替换失败保留当前版本。旧 CBC 配置密文需要重新加密，SERVER_SECRET_KEY 无默认值。

SDK 构造函数返回 error。Clone 只在客户端装配时调用，借用原连接与稳定设置；所有借用者退出后关闭原客户端。内部 Wrap 返回本次请求，不保留 Gin context。

## 验证与范围

根模块和四个独立示例执行测试与 go vet。状态、生命周期、缓存、SDK、配置、日志、加密和可选集成的并发路径执行竞态检测。生成器执行 Python 回归；生成单模块及双模块独立项目，检查服务端/CLI 构建、SQLite CRUD、幂等重放与 Swagger 文档。最小 HTTP 服务验证启动、/hello 响应及信号退出。

数据库默认配置以本地 SQLite 实测。Redis、MongoDB 验证缺失配置和取消初始化，不发布失败客户端；Dapr 使用本地 gRPC 测试服务验证发布、取消与连接关闭。没有验证真实 Redis/MongoDB 的成功读写，也没有连接真实 Kubernetes 集群或外部鉴权服务。公开下载构建验证使用发布版本，不依赖本地 replace。
