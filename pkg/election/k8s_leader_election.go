// 主从选举模块
package election

import (
	"context"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/FlyTOmeLight/server-go/logger"
	"github.com/google/uuid"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
)

// 基于k8s实现的选举
type K8sLeaderElection struct {
	/**********  配置参数  ***************/
	// 选举使用的锁所在的namespace
	namespace string
	// 当前节点的id
	id string
	// app名称，将用于选举的锁的名称，跨应用不可重复
	appName string
	// kubeconfig的文件路径, 默认使用inClusterConfig
	kubeConfigFilePath string
	// leader超时时间, 默认30s， renewDeadline被设置成了与leaseDuration相同
	leaseDuration time.Duration
	/************************************/

	/**********  回调函数  ****************/
	// 成为leader之后的回调
	startLeaderCallback func(context.Context)
	// 失去leader之后的回调
	stopLeaderCallback func()
	// leader切换时的回调
	leaderChangeCallback func(string)
	/******************/

	leaderElector *leaderelection.LeaderElector
	client        *clientset.Clientset

	// 在leader上运行的任务列表
	leaderTasks []func(context.Context)
	// 选举已经启动
	started atomic.Bool
}

type K8sElectionOption func(e *K8sLeaderElection)

func NewK8sLeaderElection(appName string, options ...K8sElectionOption) *K8sLeaderElection {
	e := &K8sLeaderElection{
		namespace:     "",
		id:            "",
		appName:       appName,
		leaseDuration: 3 * time.Second,
	}
	for _, option := range options {
		option(e)
	}
	return e
}

func NewLeaderElection(appName string, options ...K8sElectionOption) LeaderElection {
	ctx := context.Background()
	// 优先使用k8s的election
	e := NewK8sLeaderElection(appName, options...)
	if e.id == "" {
		// 未传入id时尝试自动获取ip地址作为自己的id
		// 若ip地址获取失败，则使用随机字符串作为id
		ip := GetLocalIP("")
		if ip == "" {
			e.id = uuid.New().String()
		} else {
			e.id = strings.ReplaceAll(ip, ".", "-")
		}
		logger.Infof(ctx, "AppElection id is not set, use %s as id", e.id)
	}

	var err error
	// 检查namespace是否存在，如果不存在降级为DummyAppElection
	if e.namespace == "" {
		e.namespace, err = GetCurrentNamespace()
		// 获取失败，表示当前不在k8s中，改为使用DummyAppElection
		if err != nil {
			logger.Warnf(ctx, "not in k8s, use DummyAppElection instead")
			return NewDummyAppElection(e.id)
		}
	}
	return e
}

func (e *K8sLeaderElection) createClient() {
	// 使用inClusterConfig
	var cfg *rest.Config
	var err error
	if e.kubeConfigFilePath == "" {
		cfg, err = rest.InClusterConfig()
		if err != nil {
			panic(err)
		}
	} else {
		cfg, err = clientcmd.BuildConfigFromFlags("", e.kubeConfigFilePath)
		if err != nil {
			panic(err)
		}

	}
	e.client, err = clientset.NewForConfig(cfg)
	if err != nil {
		panic(err)
	}
}

func (e *K8sLeaderElection) startElection(ctx context.Context) {
	// 创建k8s的client
	e.createClient()
	lock := &resourcelock.LeaseLock{
		LeaseMeta: metav1.ObjectMeta{
			Name:      "app-election-" + e.appName,
			Namespace: e.namespace,
		},
		Client: e.client.CoordinationV1(),
		LockConfig: resourcelock.ResourceLockConfig{
			Identity: e.id,
		},
	}
	var err error
	// leader过期时间最小值设置3s，防止频繁的leader切换
	if e.leaseDuration < 3*time.Second {
		e.leaseDuration = 3 * time.Second
	}
	// 到达renewDeadline之后，leader放弃继续renew
	renewDeadline := e.leaseDuration - 1*time.Second

	e.leaderElector, err = leaderelection.NewLeaderElector(leaderelection.LeaderElectionConfig{
		Lock:            lock,
		LeaseDuration:   e.leaseDuration,
		RenewDeadline:   renewDeadline,
		RetryPeriod:     1 * time.Second,
		ReleaseOnCancel: true,
		Callbacks:       e.leaderCallbacks(ctx),
	})
	if err != nil {
		panic(err)
	}
	// 开始选举，Run会一直执行到ctx被取消
	e.leaderElector.Run(ctx)
}

// leaderCallbacks joins every owned leader task before a term finishes. The
// upstream elector dispatches OnStartedLeading asynchronously, so the mutex also
// prevents a late callback from starting work after OnStoppedLeading.
func (e *K8sLeaderElection) leaderCallbacks(runCtx context.Context) leaderelection.LeaderCallbacks {
	var mu sync.Mutex
	var stopped bool
	var cancel context.CancelFunc
	var done chan struct{}
	return leaderelection.LeaderCallbacks{
		OnStartedLeading: func(ctx context.Context) {
			mu.Lock()
			if stopped || ctx.Err() != nil {
				mu.Unlock()
				return
			}
			ctx, cancel = context.WithCancel(ctx)
			done = make(chan struct{})
			finished := done
			mu.Unlock()
			defer close(finished)
			logger.Infof(ctx, "%s became leader now", e.id)
			if e.startLeaderCallback != nil {
				e.startLeaderCallback(ctx)
			}
			var tasks sync.WaitGroup
			for _, task := range e.leaderTasks {
				if ctx.Err() != nil {
					break
				}
				tasks.Add(1)
				go func() {
					defer tasks.Done()
					task(ctx)
				}()
			}
			tasks.Wait()
		},
		OnStoppedLeading: func() {
			mu.Lock()
			stopped = true
			stop, finished := cancel, done
			mu.Unlock()
			if stop != nil {
				stop()
				<-finished
			}
			logger.Warnf(runCtx, "%s lost leader", e.id)
			if e.stopLeaderCallback != nil {
				e.stopLeaderCallback()
			}
		},
		OnNewLeader: func(identity string) {
			logger.Infof(runCtx, "%s: new leader is %s", e.id, identity)
			if e.leaderChangeCallback != nil {
				e.leaderChangeCallback(identity)
			}
		},
	}
}

// 启动选举，只能执行一次，多次执行后续执行无效
func (e *K8sLeaderElection) RunAsync(ctx context.Context) {
	go e.Run(ctx)
}

func (e *K8sLeaderElection) Run(ctx context.Context) {
	if ctx.Err() != nil {
		return
	}
	if !e.started.CompareAndSwap(false, true) {
		logger.Warnf(ctx, "app election already started")
		return
	}

	for {
		e.startElection(ctx)
		select {
		case <-ctx.Done():
			// 取消选举，退出
			logger.Warnf(ctx, "app election canceled")
			return
		default:
			// 没有被取消，继续尝试竞选
			logger.Warnf(ctx, "app election failed, retrying...")
			continue
		}
	}

}

// 设置kubeconfig文件路径, 默认使用inClusterConfig
func WithKubeConfigFilePath(kubeConfigFilePath string) K8sElectionOption {
	return func(e *K8sLeaderElection) {
		e.kubeConfigFilePath = kubeConfigFilePath
	}
}

func WithLeaseDuration(d time.Duration) K8sElectionOption {
	return func(e *K8sLeaderElection) {
		e.leaseDuration = d
	}
}

// 设置选举的id，默认使用当前IP地址
func WithIdentify(id string) K8sElectionOption {
	return func(e *K8sLeaderElection) {
		e.id = id
	}
}

// 设置选举的namespace，默认自动获取当前程序所在的namespace
func WithNamespace(namespace string) K8sElectionOption {
	return func(e *K8sLeaderElection) {
		e.namespace = namespace
	}
}

// 添加成为leader的回调
func (e *K8sLeaderElection) WithStartedLeadingCallback(cb func(context.Context)) LeaderElection {
	currCallback := e.startLeaderCallback
	e.startLeaderCallback = func(ctx context.Context) {
		if currCallback != nil {
			currCallback(ctx)
		}
		cb(ctx)
	}
	return e
}

// 添加时区leader的回调
func (e *K8sLeaderElection) WithStoppedLeadingCallback(cb func()) LeaderElection {
	currCallback := e.stopLeaderCallback
	e.stopLeaderCallback = func() {
		if currCallback != nil {
			currCallback()
		}
		cb()
	}
	return e
}

// 添加leader变化的回调
func (e *K8sLeaderElection) WithNewLeaderCallback(cb func(string)) LeaderElection {
	currCallback := e.leaderChangeCallback
	e.leaderChangeCallback = func(newLeader string) {
		if currCallback != nil {
			currCallback(newLeader)
		}
		cb(newLeader)
	}
	return e
}

// 获取当前Leader
func (e *K8sLeaderElection) GetLeader() string {
	if e.leaderElector != nil {
		return e.leaderElector.GetLeader()
	}
	return "no leader"
}

// 判断自己是否是leader
func (e *K8sLeaderElection) IsLeader() bool {
	if e.leaderElector == nil {
		return false
	}
	return e.leaderElector.IsLeader()
}

// RunOnLeader需要在Run()执行之前调用，否则不会被执行
func (e *K8sLeaderElection) RunOnLeader(task func(context.Context)) {
	e.leaderTasks = append(e.leaderTasks, task)
}
