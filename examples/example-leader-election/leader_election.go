// 一个示例代码，用来演示如何使用LeaderElection，怎样只在主节点上运行任务
package main

import (
	"context"
	"flag"
	"fmt"
	"time"

	"github.com/FlyTOmeLight/server-go/pkg/election"
)

// 模拟一个任务执行函数
func task(ctx context.Context) {
	// 执行任务，任务重通常包含循环，可以在循环中判断context是否结束，来终止任务
	// 不包含循环的，需要在安全的时候尽快退出任务
	for i := 0; ; i++ {
		select {
		case <-ctx.Done():
			fmt.Println("task is cancelled")
			return
		default:
			// 实际要进行的迭代操作
			fmt.Println(i)
			time.Sleep(time.Second)
		}
	}
}

func main() {
	/*
	*   示例运行： go run leader_election.go --kubeconfig ~/.kube/dolphintest.conf -namespace dolphin -time 20s -id ins2 -app-name testapp1
	 */
	// 指定kubeconfig文件路径, k8s环境不需要指定，会默认使用InClusterConfig
	kubeconfigPath := flag.String("kubeconfig", "", "kubeconfig file path")
	// 选举进行的namespace，k8s运行中不需要指定，会自动获取当前程序所在的namespace
	namespace := flag.String("namespace", "", "namespace")
	// 选举的app名称，通常为应用名称
	appName := flag.String("app-name", "testapp", "app name")
	// 丢失心跳后，leader超时时间
	leaseDuration := flag.Duration("ttl", time.Second*10, "lease duration")
	// 选举的id，如果为空，将先获取当前IP地址，获取不到则使用uuid
	id := flag.String("id", "", "election id")
	// 程序运行持续时间
	runTime := flag.Duration("time", time.Second*5, "run time")
	flag.Parse()

	e := election.NewLeaderElection(*appName,
		election.WithKubeConfigFilePath(*kubeconfigPath),
		election.WithLeaseDuration(*leaseDuration),
		election.WithNamespace(*namespace),
		election.WithIdentify(*id))
	// 必须在Run之前执行，否则不会被运行
	e.RunOnLeader(task)
	// leader切换回调
	e.WithNewLeaderCallback(func(leader string) {
		fmt.Println("new leader is: ", leader)
	})
	// 失去leader回调, 注意这个回调在主动cancel时也会被调用
	e.WithStoppedLeadingCallback(func() {
		fmt.Println("leader is lost, current: ", e.GetLeader())
	})
	// 成为leader回调
	e.WithStartedLeadingCallback(func(ctx context.Context) {
		fmt.Println("I am leader")
	})
	// 只运行5s钟，这样可以看到task被取消
	ctx, cancel := context.WithTimeout(context.Background(), *runTime)
	defer cancel()
	e.Run(ctx)
	<-ctx.Done()
	// 等待2s再退出，这样可以看到选举取消
	time.Sleep(2 * time.Second)
}
