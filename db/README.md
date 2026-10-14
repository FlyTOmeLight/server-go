# 数据库事务

`db.Transaction(ctx, database, options, callback)` 在传入的连接池上执行完整的 GORM 事务。成功提交，回调返回错误或 panic 时回滚，panic 继续传播。SQL 隔离级别与只读选项由调用方通过 `TransactionOptions` 显式选择，零值使用数据库默认值。

```go
err := db.Transaction(ctx, database, db.TransactionOptions{
    Isolation: sql.LevelReadCommitted,
    Retry: &db.TransactionRetry{
        MaxAttempts: 3,
        Timeout:     5 * time.Second,
        Delay:       20 * time.Millisecond,
    },
}, func(tx *db.DB) error {
    // 每次尝试重新创建查询结果和写入对象，并始终通过 tx 查询。
    return tx.Exec("UPDATE accounts SET balance = balance + ? WHERE id = ?", amount, id).Error
})
```

重试默认关闭。启用时必须同时给出正的 `MaxAttempts` 和 `Timeout`，次数包含首次执行，时间预算覆盖所有尝试与等待。上游 context 更早取消或到期时，以它为准。回调必须使用传入的事务句柄及其 context；context 无法强制停止忽略取消的 Go 回调。不要在回调内手工调用 Commit 或 Rollback。

只有 PostgreSQL `40001`（序列化冲突）、`40P01`（死锁）和 MySQL InnoDB `1213`（死锁）这类数据库已明确终止事务的错误允许重试。每次重试重新开启整个事务，获得新的句柄。MySQL 锁等待超时 `1205`、唯一键冲突、业务错误、连接错误，以及提交结果未知的错误直接返回。框架保留原始错误链；context 取消时同时保留 context 错误与数据库错误，调用方可用 `errors.Is` / `errors.As` 判断。不要先把驱动错误转换为 HTTP 错误再返回给事务回调。

本 API 拒绝嵌套调用。内部函数接收外层 `tx`，不要再次开启事务。如果直接使用 GORM 的嵌套事务，它遵循 GORM 保存点语义；保存点回滚不能代替整笔事务的死锁或序列化冲突重试。

回调只包含可以回滚的数据库操作。每次尝试重建中间结果，不累加上次尝试的数据。外部 HTTP、对象存储和消息发送放在事务外；事务重试不能撤销这些副作用。业务事务范围、锁顺序和幂等规则由业务服务决定。

[可运行示例](transaction_example_test.go) 使用独立 SQLite 数据库演示提交。`make check` 创建并清理独立 PostgreSQL 17 和 MySQL 8.4 容器，执行双库提交、回滚、panic、只读事务、唯一键冲突、取消查询与真实死锁重试测试。直接运行 `go test ./db` 时，没有对应测试 DSN 的双库子测试会跳过；完整交付以 `make check` 为准。

重试依据：[PostgreSQL 序列化失败处理](https://www.postgresql.org/docs/17/mvcc-serialization-failure-handling.html)要求重跑完整事务；[MySQL InnoDB 错误处理](https://dev.mysql.com/doc/refman/8.4/en/innodb-error-handling.html)说明死锁回滚整个事务，锁等待超时默认只回滚当前语句。MySQL 业务表须采用支持事务的 InnoDB 引擎。
