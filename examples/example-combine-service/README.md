# 组合服务

把 HelloWorld、数据集和无状态 HTTP 服务装入同一个 HTTPHost，三个服务都通过公开 `service` 包导入。HelloWorld 与数据集共享一个 SQLite 数据库，组合应用拥有数据库生命周期，不重复初始化数据库。

从本目录运行：

```sh
go mod tidy
make run
```

可访问 `/helloworld/internal/v1/*Module`、`/v1/projects/:projectName/datasets` 和 `/service2/hello`。请求示例见 [HelloWorld](../example-hello-world/README.md)、[数据集](../example-service1/README.md) 和 [无状态服务](../example-service2/README.md)。

`cmd/server` 只负责进程入口；类型化配置与三个服务的装配在 `service/build.go`。无需复制被组合项目的 `internal` 实现，也无需初始化未使用的 MongoDB。`DB.AutoMigrate` 同时传入两个数据库服务的构造函数。应用退出时先停止服务，再关闭共享数据库。

`cmd/cli` 组合已有的 `module` 与 `dataset` 命令：

```sh
make build
./output/bin/combine config set core.endpoint=http://127.0.0.1:8080
./output/bin/combine module create --project-id=project1 --name=example
./output/bin/combine dataset list --project-name=project1
```

`make check` 包含共享数据库和路由组合测试。各模块由本地 `replace` 连接；配置、打包和版本约定见 [示例索引](../README.md)。
