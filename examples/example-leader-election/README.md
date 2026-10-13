# Leader 选举 worker

本示例属于根模块，展示只在 leader 上运行的后台任务。入口在 `cmd/worker`，公开 `service` 构造选举实例并拥有同步运行的生命周期，实际任务在 `internal/app/leadership/task.go`。worker 使用标准 `context.Context`，取消后等待任务退出。

不连接 Kubernetes 的本地演示：

```sh
go run ./cmd/worker --local --id=local --time=3s
```

显式使用 Kubernetes 选举：

```sh
go run ./cmd/worker --kubeconfig="$HOME/.kube/config" --namespace=default \
  --app-name=example-worker --id=instance1 --time=20s
```

在集群中运行时可省略 kubeconfig 和 namespace。指定 kubeconfig 后，未指定的 namespace 使用其当前 context 的 namespace（未配置时为 `default`）。`--ttl` 控制 lease 时长，`--time` 控制演示持续时间；Ctrl+C 或 SIGTERM 也会取消任务。`--local` 明确选择单节点实现，不执行跨副本选举。Kubernetes 配置或初始化失败会报告错误并以非零状态退出，不会自动切换为本地 leader。

`election.NewLeaderElection` 和 `election.NewK8sLeaderElection` 返回实例与错误；本地运行显式调用 `election.NewDummyAppElection`。同步 `Run(ctx)` 返回初始化错误，异步 `RunAsync(ctx)` 返回供调用方等待结果的 channel。

worker 不监听 HTTP，也不创建空 HTTPHost 或 HTTP 配置。类型化 `service.Config` 由入口的命令行参数构造。`make build` 输出 `output/bin/leader-worker`，`make check` 检查格式、测试和 vet。其余目录约定见 [示例索引](../README.md)。
