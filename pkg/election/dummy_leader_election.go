package election

import (
	"context"
	"sync"

	"github.com/FlyTOmeLight/server-go/logger"
)

// 一个空的选举实现， 为了满足非k8s环境下的接口一致性
type DummyAppElection struct {
	// 当前节点ID
	ID string
	// 要执行的函数
	leaderTasks []func(context.Context)
}

func NewDummyAppElection(id string) *DummyAppElection {
	return &DummyAppElection{ID: id}
}

// 添加成为leader的回调
func (e *DummyAppElection) WithStartedLeadingCallback(cb func(context.Context)) LeaderElection {
	// 也在leaderTasks中执行，因为leaderTasks也只会被执行一次
	e.leaderTasks = append(e.leaderTasks, cb)
	return e
}

// 添加时区leader的回调
func (e *DummyAppElection) WithStoppedLeadingCallback(_ func()) LeaderElection {
	// 不需要实现，因为永远是leader
	return e
}

// 添加leader变化的回调
func (e *DummyAppElection) WithNewLeaderCallback(_ func(string)) LeaderElection {
	// 不需要实现，因为永远是leader，不会发生切换
	return e
}

// 获取当前Leader
func (e *DummyAppElection) GetLeader() string {
	// 总是自己
	return e.ID
}

// 判断自己是否是leader
func (e *DummyAppElection) IsLeader() bool {
	// 总是leader
	return true
}

func (e *DummyAppElection) RunOnLeader(task func(context.Context)) {
	e.leaderTasks = append(e.leaderTasks, task)
}

func (e *DummyAppElection) Run(ctx context.Context) {
	// The application owns cancellation. Wait for every owned task to stop.
	var tasks sync.WaitGroup
	for _, task := range e.leaderTasks {
		tasks.Add(1)
		go func() {
			defer tasks.Done()
			task(ctx)
		}()
	}
	<-ctx.Done()
	tasks.Wait()
	logger.Infof(ctx, "app election canceled")
}

func (e *DummyAppElection) RunAsync(ctx context.Context) {
	go e.Run(ctx)
}
