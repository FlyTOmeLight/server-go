package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/FlyTOmeLight/server-go/examples/example-leader-election/service"
)

func main() {
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

	local := flag.Bool("local", false, "run as a single local leader without Kubernetes")
	flag.Parse()
	signalCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(signalCtx, *runTime)
	defer cancel()
	service.New(service.Config{AppName: *appName, Kubeconfig: *kubeconfigPath, Namespace: *namespace, ID: *id, LeaseDuration: *leaseDuration, Local: *local}).
		Run(ctx)
}
