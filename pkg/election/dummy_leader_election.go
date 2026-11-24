package election

import (
	"context"
	"errors"
	"sync/atomic"

	"github.com/FlyTOmeLight/server-go/logger"
)

// 一个空的选举实现， 为了满足非k8s环境下的接口一致性
type DummyAppElection struct {
	// 当前节点ID
	ID string

	work    leaderWork
	running atomic.Bool
}

func NewDummyAppElection(id string) *DummyAppElection {
	return &DummyAppElection{ID: id}
}

// 添加成为leader的回调
func (e *DummyAppElection) WithStartedLeadingCallback(cb func(context.Context)) LeaderElection {
	if cb != nil {
		addLeaderHook(&e.work.mu, &e.work.started, cb)
	}
	return e
}

// 添加失去leader的回调
func (e *DummyAppElection) WithStoppedLeadingCallback(cb func()) LeaderElection {
	if cb != nil {
		addLeaderHook(&e.work.mu, &e.work.stopped, cb)
	}
	return e
}

// 添加leader变化的回调
func (e *DummyAppElection) WithNewLeaderCallback(cb func(string)) LeaderElection {
	if cb != nil {
		addLeaderHook(&e.work.mu, &e.work.changed, cb)
	}
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
	if task != nil {
		addLeaderHook(&e.work.mu, &e.work.tasks, task)
	}
}

func (e *DummyAppElection) Run(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if !e.running.CompareAndSwap(false, true) {
		return errors.New("app election already started")
	}
	defer e.running.Store(false)

	hooks := e.work.snapshot()
	for _, callback := range hooks.started {
		callback(ctx)
	}
	for _, callback := range hooks.changed {
		callback(e.ID)
	}
	runLeaderTasks(ctx, hooks.tasks)
	<-ctx.Done()
	for _, callback := range hooks.stopped {
		callback()
	}
	logger.Infof(ctx, "app election canceled")
	return nil
}

func (e *DummyAppElection) RunAsync(ctx context.Context) <-chan error {
	done := make(chan error, 1)
	go func() {
		done <- e.Run(ctx)
		close(done)
	}()
	return done
}
