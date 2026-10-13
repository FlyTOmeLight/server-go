# PostgreSQL notes 服务

这个示例使用仓库根模块，演示 `app.Run`、类型化配置、显式数据库注入、Controller / Service / Table 分工和公开 Go SDK。无需新建 `go.mod`。数据库由 `runtime.Initialize` 接管，退出时先排空 HTTP，再关闭连接池。

在本目录启动本地 PostgreSQL 17 与服务：

```sh
make postgres
make run
```

默认监听 `127.0.0.1:18080`，数据库连接为 Compose 中的本地示例值。`DATABASE_URL` 可覆盖完整 PostgreSQL DSN，`LISTEN_ADDR` 可覆盖监听地址；TLS、search_path 和其他参数随 DSN 交给驱动。完成后运行 `make stop`。

```sh
curl -sS http://127.0.0.1:18080/v1/notes \
  -H 'Content-Type: application/json' \
  -d '{"name":"first note","description":"hello","priority":2,"enabled":true,"reviewed":null}'
curl -sS 'http://127.0.0.1:18080/v1/notes?pageNo=1&pageSize=20'
```

接口：

| 方法与路径 | 结果 |
| --- | --- |
| `POST /v1/notes` | 创建，201 与 Note 对象 |
| `GET /v1/notes` | 分页列表，200 与 `results`、`totalCount`、`pageNo`、`pageSize` |
| `GET /v1/notes/:id` | 读取，200 与 Note 对象 |
| `PUT /v1/notes/:id` | 完整替换可写字段，200 与更新后的 Note |
| `DELETE /v1/notes/:id` | 删除，204 空响应 |

ID 是 UUID；name 必填且最多 64 字符，description 最多 1024 字符，priority 为 0..10。Controller 先用 `defaults.Set` 填入 priority=2、enabled=true，再用 `binder.Bind(...).All()` 合并请求并校验；显式传入的 0、false 会覆盖默认值。reviewed 可为 null、true、false；它映射到 `sql.NullBool`，普通 enabled 使用 PostgreSQL 原生 boolean。数值 priority 的 `tinyint` 标签映射为 SMALLINT。PUT 中省略 reviewed 或传 null 都会清空它。

列表 pageNo 默认 1，范围 1..1000000；pageSize 默认 20，范围 1..100。非法请求返回 400，不存在的 UUID 返回 404。SDK 位于 `sdk/go/notev1`，每次请求传递 context，并使用框架标准错误解码；创建请求不自动重试。

原生 SQL 使用 PostgreSQL 语法，框架不会改写字符串里的反引号。默认 `DB.AutoMigrate=true` 便于运行示例，已有 schema 时可关闭该配置。

```sh
make package OUTDIR=/tmp/postgresql-example
cd /tmp/postgresql-example
./bin/postgresql-server
```

打包目录包含 `bin/postgresql-server` 和 `conf/server/config.toml`，默认输出到 `output/`。`make build` 只构建二进制，`make run` 从源码启动，`make check` 检查本示例的格式、测试与 vet；这些命令在示例目录执行。完整的隔离数据库与 HTTP 验证由仓库根目录的 `make check` 负责。

从仓库根目录执行 `go test ./examples/postgresql-service/...` 可运行请求校验回归。真实数据库测试需要专用测试数据库 DSN：

```sh
SERVER_GO_TEST_POSTGRES_DSN='postgres://notes:notes-local@127.0.0.1:55432/notes?sslmode=disable' \
  go test -count=1 ./examples/postgresql-service/...
```

未设置该变量时真实数据库测试跳过。根目录 `make check` 自动创建隔离 PostgreSQL 容器并注入 DSN，完整检查不会跳过；测试通过 SDK 验证 CRUD、默认值、分页、NULL/true/false、原生 SQL、取消与资源关闭。
