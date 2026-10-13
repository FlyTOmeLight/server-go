# 业务项目开发

新增或修改接口前，先读 [README.md](README.md) 中的开发流程。沿用现有领域包、SDK 契约与显式装配；框架行为以所引用版本的公开接口为准。

完成修改后执行 `make check`。修改接口时同步更新请求/响应、路由、SDK、CLI 和 Swagger；通过 SDK 的实际 HTTP 请求验证行为。
