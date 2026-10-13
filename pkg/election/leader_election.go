// 用来处理微服务多副本的leader选举；本地单节点运行需显式选择 DummyAppElection。
package election

import "context"

// 主节点选举
type LeaderElection interface {
	// 添加成为leader的回调
	WithStartedLeadingCallback(cb func(context.Context)) LeaderElection

	// 添加时区leader的回调
	WithStoppedLeadingCallback(cb func()) LeaderElection

	// 添加leader变化的回调
	WithNewLeaderCallback(cb func(string)) LeaderElection

	// 获取当前Leader
	GetLeader() string

	// 判断自己是否是leader
	IsLeader() bool

	// 注册一个函数，当成为leader的时候执行，当失去leader的时候取消
	// 注意：
	// 1. 函数中需要监听context.Done()，当为true的时候，说明已经失去leader了
	// 2. 调用方需要确保cancel的时候要立刻停止运行，否则可能会存在多个副本上都在运行的情况
	// 3. 必须在Run()之前调用，否则注册的任务不会被实际执行
	RunOnLeader(func(context.Context))

	// 开始选举，同步运行，会阻塞程序，在失去leader的时候会继续尝试竞选成为leader
	Run(ctx context.Context) error

	// 开始异步选举。调用方通过返回的 channel 等待结束并接收初始化错误。
	RunAsync(ctx context.Context) <-chan error
}
