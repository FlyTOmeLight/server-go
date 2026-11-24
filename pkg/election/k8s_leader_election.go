// 主从选举模块
package election

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"

	"github.com/FlyTOmeLight/server-go/logger"
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

	leaseDuration    time.Duration
	renewDeadline    time.Duration
	retryPeriod      time.Duration
	leaseDurationSet bool
	renewDeadlineSet bool
	retryPeriodSet   bool

	mu            sync.RWMutex
	leaderElector *leaderelection.LeaderElector
	client        *clientset.Clientset
	work          leaderWork

	// 选举已经启动
	started atomic.Bool
}

type K8sElectionOption func(e *K8sLeaderElection)

// NewK8sLeaderElection validates Kubernetes configuration without starting an election.
func NewK8sLeaderElection(appName string, options ...K8sElectionOption) (*K8sLeaderElection, error) {
	e := &K8sLeaderElection{
		appName:       appName,
		leaseDuration: 15 * time.Second,
		renewDeadline: 10 * time.Second,
		retryPeriod:   2 * time.Second,
	}
	for _, option := range options {
		option(e)
	}
	if e.appName == "" || len(validation.IsDNS1123Subdomain("app-election-"+e.appName)) > 0 {
		return nil, errors.New("app name must form a valid Kubernetes lease name")
	}
	if e.leaseDurationSet {
		if !e.renewDeadlineSet {
			e.renewDeadline = e.leaseDuration - e.leaseDuration/3
		}
		if !e.retryPeriodSet {
			e.retryPeriod = e.renewDeadline / 5
		}
	}
	// Kubernetes stores lease duration in whole seconds.
	if e.leaseDuration < time.Second {
		return nil, fmt.Errorf(
			"invalid leader election lease duration %s: must be at least one second",
			e.leaseDuration,
		)
	}
	if e.retryPeriod <= 0 || e.leaseDuration.Truncate(time.Second) <= e.renewDeadline ||
		float64(e.renewDeadline) <= leaderelection.JitterFactor*float64(e.retryPeriod) {
		return nil, fmt.Errorf(
			"invalid leader election timings: lease duration (%s) must exceed renew deadline (%s), "+
				"which must exceed %g times the positive retry period (%s)",
			e.leaseDuration,
			e.renewDeadline,
			leaderelection.JitterFactor,
			e.retryPeriod,
		)
	}
	if e.id == "" {
		e.id = defaultIdentity()
	}
	if err := e.createClient(); err != nil {
		return nil, err
	}
	return e, nil
}

func defaultIdentity() string {
	node := os.Getenv("POD_NAME")
	if node == "" {
		node, _ = os.Hostname()
	}
	if node == "" {
		if ip, err := LookupLocalIP(""); err == nil {
			node = strings.ReplaceAll(ip, ".", "-")
		}
	}
	id := uuid.New().String()
	if node == "" {
		return id
	}
	return node + "-" + id[:8]
}

// NewLeaderElection always selects Kubernetes. Use NewDummyAppElection explicitly for local work.
func NewLeaderElection(appName string, options ...K8sElectionOption) (LeaderElection, error) {
	e, err := NewK8sLeaderElection(appName, options...)
	if err != nil {
		return nil, err
	}
	return e, nil
}

func (e *K8sLeaderElection) createClient() error {
	// 使用inClusterConfig
	var cfg *rest.Config
	var err error
	if e.kubeConfigFilePath == "" {
		cfg, err = rest.InClusterConfig()
		if err != nil {
			return fmt.Errorf("load in-cluster Kubernetes configuration: %w", err)
		}
		if e.namespace == "" {
			e.namespace, err = GetCurrentNamespace()
			if err != nil {
				return fmt.Errorf("load Kubernetes namespace: %w", err)
			}
		}
	} else {
		configuration := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
			&clientcmd.ClientConfigLoadingRules{
				ExplicitPath: e.kubeConfigFilePath,
			},
			&clientcmd.ConfigOverrides{},
		)
		cfg, err = configuration.ClientConfig()
		if err != nil {
			return fmt.Errorf("load kubeconfig: %w", err)
		}
		if e.namespace == "" {
			e.namespace, _, err = configuration.Namespace()
			if err != nil {
				return fmt.Errorf("load kubeconfig namespace: %w", err)
			}
		}
	}
	if len(validation.IsDNS1123Label(e.namespace)) > 0 {
		return fmt.Errorf("invalid Kubernetes namespace %q", e.namespace)
	}
	e.client, err = clientset.NewForConfig(cfg)
	if err != nil {
		return fmt.Errorf("create Kubernetes client: %w", err)
	}
	return nil
}

func (e *K8sLeaderElection) startElection(ctx context.Context) (bool, error) {
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
	var led atomic.Bool
	elector, err := leaderelection.NewLeaderElector(leaderelection.LeaderElectionConfig{
		Lock:            lock,
		LeaseDuration:   e.leaseDuration,
		RenewDeadline:   e.renewDeadline,
		RetryPeriod:     e.retryPeriod,
		ReleaseOnCancel: true,
		Callbacks:       e.leaderCallbacks(ctx, func() { led.Store(true) }),
	})
	if err != nil {
		return false, fmt.Errorf("initialize Kubernetes leader election: %w", err)
	}
	e.mu.Lock()
	e.leaderElector = elector
	e.mu.Unlock()
	elector.Run(ctx)
	e.mu.Lock()
	e.leaderElector = nil
	e.mu.Unlock()
	return led.Load(), nil
}

// leaderCallbacks joins every owned leader task before a term finishes. The
// upstream elector dispatches OnStartedLeading asynchronously, so the mutex also
// prevents a late callback from starting work after OnStoppedLeading.
func (e *K8sLeaderElection) leaderCallbacks(runCtx context.Context, onStarted func()) leaderelection.LeaderCallbacks {
	hooks := e.work.snapshot()
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
			if onStarted != nil {
				onStarted()
			}
			logger.Infof(ctx, "%s became leader now", e.id)
			for _, callback := range hooks.started {
				callback(ctx)
			}
			runLeaderTasks(ctx, hooks.tasks)
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
			for _, callback := range hooks.stopped {
				callback()
			}
		},
		OnNewLeader: func(identity string) {
			logger.Infof(runCtx, "%s: new leader is %s", e.id, identity)
			for _, callback := range hooks.changed {
				callback(identity)
			}
		},
	}
}

// 启动选举，只能执行一次，多次执行后续执行无效
func (e *K8sLeaderElection) RunAsync(ctx context.Context) <-chan error {
	done := make(chan error, 1)
	go func() {
		done <- e.Run(ctx)
		close(done)
	}()
	return done
}

func (e *K8sLeaderElection) Run(ctx context.Context) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if !e.started.CompareAndSwap(false, true) {
		return errors.New("app election already started")
	}

	return runElection(ctx, e.startElection)
}

func runElection(ctx context.Context, run func(context.Context) (bool, error)) error {
	const initialDelay, maxDelay = 500 * time.Millisecond, 5 * time.Second
	delay := initialDelay
	for ctx.Err() == nil {
		led, err := run(ctx)
		if err != nil {
			return err
		}
		if ctx.Err() != nil {
			break
		}
		if led {
			delay = initialDelay
		}
		logger.Warnf(ctx, "app election ended, retrying in %s", delay)
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
		case <-timer.C:
			delay = min(delay*2, maxDelay)
		}
	}
	logger.Warnf(ctx, "app election canceled")
	return nil
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
		e.leaseDurationSet = true
	}
}

// WithRenewDeadline sets how long the leader retries renewal before stepping down.
func WithRenewDeadline(d time.Duration) K8sElectionOption {
	return func(e *K8sLeaderElection) {
		e.renewDeadline = d
		e.renewDeadlineSet = true
	}
}

// WithRetryPeriod sets the interval between acquisition or renewal attempts.
func WithRetryPeriod(d time.Duration) K8sElectionOption {
	return func(e *K8sLeaderElection) {
		e.retryPeriod = d
		e.retryPeriodSet = true
	}
}

// WithIdentify sets the identity recorded on the lease.
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
	if cb != nil {
		addLeaderHook(&e.work.mu, &e.work.started, cb)
	}
	return e
}

// 添加失去leader的回调
func (e *K8sLeaderElection) WithStoppedLeadingCallback(cb func()) LeaderElection {
	if cb != nil {
		addLeaderHook(&e.work.mu, &e.work.stopped, cb)
	}
	return e
}

// 添加leader变化的回调
func (e *K8sLeaderElection) WithNewLeaderCallback(cb func(string)) LeaderElection {
	if cb != nil {
		addLeaderHook(&e.work.mu, &e.work.changed, cb)
	}
	return e
}

// 获取当前Leader
func (e *K8sLeaderElection) GetLeader() string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	if e.leaderElector != nil {
		return e.leaderElector.GetLeader()
	}
	return "no leader"
}

// 判断自己是否是leader
func (e *K8sLeaderElection) IsLeader() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	if e.leaderElector == nil {
		return false
	}
	return e.leaderElector.IsLeader()
}

// RunOnLeader需要在Run()执行之前调用，否则不会被执行
func (e *K8sLeaderElection) RunOnLeader(task func(context.Context)) {
	if task != nil {
		addLeaderHook(&e.work.mu, &e.work.tasks, task)
	}
}
