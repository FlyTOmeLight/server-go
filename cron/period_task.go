package cron

import (
	"context"
	"errors"
	"fmt"
	"runtime/debug"
	"sync"
	"time"

	"github.com/panjf2000/ants/v2"
	"github.com/robfig/cron/v3"

	"github.com/FlyTOmeLight/server-go/logger"
)

type Task interface {
	Name() string
	Init(context.Context) error
	Start(context.Context) error
	Stop(context.Context) error
}

type TaskStatus string

const (
	TaskStatusInit    TaskStatus = "Init"
	TaskStatusRunning TaskStatus = "Running"
	TaskStatusStopped TaskStatus = "Stopped"
	TaskStatusFailed  TaskStatus = "Failed"
)

type UpdatePolicy string

const (
	AutoUpdate      UpdatePolicy = "Auto"
	ForbiddenUpdate UpdatePolicy = "Forbidden"
	Recreate        UpdatePolicy = "Recreate"
)

type SyncFunc func(context.Context) (map[string]*PeriodTask, error)

type TaskManager struct {
	mu           sync.RWMutex
	updateMu     sync.Mutex
	execMu       sync.Mutex
	tasks        map[string]*managedTask
	scheduler    *cron.Cron
	pool         *ants.PoolWithFunc
	autoSync     bool
	syncHandlers []SyncFunc
	errorHandler func(error)
	active       sync.WaitGroup
	started      bool
	stopping     bool
	runCtx       context.Context
	cancel       context.CancelFunc
	stop         *stopAttempt
}

type managedTask struct {
	name       string
	period     *PeriodTask
	initFailed bool
}

type stopAttempt struct {
	done chan struct{}
	err  error
}

const taskCleanupTimeout = 5 * time.Second

func NewTaskManager(interval time.Duration, concurrency int, autoSync bool) (*TaskManager, error) {
	if interval < time.Second || interval%time.Second != 0 {
		return nil, errors.New("task interval must be a positive whole number of seconds")
	}
	if concurrency == 0 {
		concurrency = 1
	}
	if concurrency < 0 {
		return nil, errors.New("task concurrency must be positive")
	}
	manager := &TaskManager{
		tasks: make(map[string]*managedTask), scheduler: cron.New(),
		autoSync: autoSync,
	}
	pool, err := ants.NewPoolWithFunc(concurrency, func(payload any) {
		defer func() {
			if failure := recover(); failure != nil {
				// The pool's panic handler reports the panic before marking this job done.
				panic(failure)
			}
			manager.active.Done()
		}()
		task := payload.(*managedTask)
		if err := manager.execTask(task); err != nil {
			manager.reportError(fmt.Errorf("execute task %s: %w", task.name, err))
		}
	}, ants.WithPanicHandler(func(failure any) {
		defer manager.active.Done()
		manager.reportError(fmt.Errorf("task worker panic: %v\n%s", failure, debug.Stack()))
	}))
	if err != nil {
		return nil, fmt.Errorf("initialize task pool: %w", err)
	}
	manager.pool = pool
	manager.scheduler.Schedule(cron.Every(interval), cron.FuncJob(func() {
		manager.Exec(manager.runCtx)
	}))
	return manager, nil
}

// SetErrorHandler installs a synchronous handler for scheduler errors. Reports
// may come from concurrent workers. A nil handler restores logging.
func (m *TaskManager) SetErrorHandler(fn func(error)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.errorHandler = fn
}

func (m *TaskManager) reportError(err error) {
	if err == nil {
		return
	}
	m.mu.RLock()
	handler, ctx := m.errorHandler, m.runCtx
	m.mu.RUnlock()
	if handler != nil {
		handler(err)
		return
	}
	logger.FromContext(ctx).Errorf(ctx, "task manager: %v", err)
}

func (m *TaskManager) RegisterSyncHandler(handler SyncFunc) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.stopping {
		return errors.New("task manager is stopping")
	}
	if handler == nil {
		return errors.New("sync handler is nil")
	}
	m.syncHandlers = append(m.syncHandlers, handler)
	return nil
}

func (m *TaskManager) Add(ctx context.Context, task *PeriodTask) error {
	if task == nil || task.task == nil {
		return errors.New("task and its name must be provided")
	}
	name := task.task.Name()
	if name == "" {
		return errors.New("task and its name must be provided")
	}
	m.updateMu.Lock()
	defer m.updateMu.Unlock()
	return m.add(ctx, name, task, false)
}

// add requires updateMu to be held through lookup, cleanup, and publication.
func (m *TaskManager) add(ctx context.Context, name string, task *PeriodTask, syncing bool) error {
	switch task.updatePolicy {
	case AutoUpdate, ForbiddenUpdate, Recreate:
	default:
		return fmt.Errorf("unknown task update policy: %s", task.updatePolicy)
	}
	m.mu.RLock()
	old, stopping := m.tasks[name], m.stopping
	m.mu.RUnlock()
	if stopping {
		return errors.New("task manager is stopping")
	}
	if old != nil && old.initFailed {
		if err := m.cleanupFailedInit(ctx, old); err != nil {
			return err
		}
	} else if old != nil {
		switch task.updatePolicy {
		case ForbiddenUpdate:
			if syncing {
				return nil
			}
			return errors.New("task already exists")
		case AutoUpdate:
			old.period.updateSchedule(task)
			return nil
		case Recreate:
			if err := m.remove(ctx, old); err != nil {
				return err
			}
		}
	}
	entry := &managedTask{name: name, period: task}
	if err := task.init(ctx); err != nil {
		entry.initFailed = true
		if cleanupErr := m.cleanupFailedInit(ctx, entry); cleanupErr != nil {
			m.mu.Lock()
			m.tasks[name] = entry
			m.mu.Unlock()
			return errors.Join(err, cleanupErr)
		}
		return err
	}
	m.mu.Lock()
	m.tasks[name] = entry
	m.mu.Unlock()
	return nil
}

func (m *TaskManager) SyncTasks(ctx context.Context, tasks map[string]*PeriodTask, syncErr error) error {
	err := m.syncTasks(ctx, tasks, syncErr)
	m.reportError(err)
	return err
}

func (m *TaskManager) syncTasks(ctx context.Context, tasks map[string]*PeriodTask, syncErr error) error {
	if syncErr != nil {
		return syncErr
	}
	m.updateMu.Lock()
	defer m.updateMu.Unlock()
	m.mu.RLock()
	stopping := m.stopping
	m.mu.RUnlock()
	if stopping {
		return errors.New("task manager is stopping")
	}
	var errs []error
	for name, task := range tasks {
		if name == "" || task == nil || task.task == nil || task.task.Name() != name {
			errs = append(errs, fmt.Errorf("invalid synced task: %s", name))
			continue
		}
		if err := m.add(ctx, name, task, true); err != nil {
			errs = append(errs, fmt.Errorf("sync task %s: %w", name, err))
		}
	}
	m.mu.RLock()
	var deleted []*managedTask
	for name, task := range m.tasks {
		if _, exists := tasks[name]; !exists {
			deleted = append(deleted, task)
		}
	}
	m.mu.RUnlock()
	for _, task := range deleted {
		if err := m.remove(ctx, task); err != nil {
			errs = append(errs, fmt.Errorf("remove task %s: %w", task.name, err))
		}
	}
	return errors.Join(errs...)
}

func (m *TaskManager) GetPeriodTask(name string) *PeriodTask {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if task := m.tasks[name]; task != nil {
		return task.period
	}
	return nil
}

func (m *TaskManager) GetTask(name string) Task {
	if task := m.GetPeriodTask(name); task != nil {
		return task.task
	}
	return nil
}

func (m *TaskManager) Remove(ctx context.Context, name string) error {
	m.updateMu.Lock()
	defer m.updateMu.Unlock()
	m.mu.RLock()
	task := m.tasks[name]
	m.mu.RUnlock()
	if task == nil {
		return nil
	}
	return m.remove(ctx, task)
}

func (m *TaskManager) remove(ctx context.Context, task *managedTask) error {
	closeTask := task.period.close
	if task.initFailed {
		closeTask = task.period.cleanupFailedInit
	}
	if err := closeTask(ctx); err != nil {
		return err
	}
	m.mu.Lock()
	if m.tasks[task.name] == task {
		delete(m.tasks, task.name)
	}
	m.mu.Unlock()
	return nil
}

// cleanupFailedInit requires updateMu. A failed Init stays owned only while it
// still needs cleanup.
func (m *TaskManager) cleanupFailedInit(ctx context.Context, task *managedTask) error {
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), taskCleanupTimeout)
	defer cancel()
	return m.remove(cleanupCtx, task)
}

func (m *TaskManager) execTask(task *managedTask) error {
	if !task.initFailed {
		return task.period.Exec(m.runCtx)
	}
	m.updateMu.Lock()
	defer m.updateMu.Unlock()
	m.mu.RLock()
	current := m.tasks[task.name]
	m.mu.RUnlock()
	if current != task {
		return nil
	}
	return m.cleanupFailedInit(m.runCtx, task)
}

func (m *TaskManager) Start(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.stopping {
		return errors.New("task manager is stopping")
	}
	if m.started {
		return nil
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	m.runCtx, m.cancel = context.WithCancel(ctx)
	m.started = true
	m.scheduler.Start()
	return nil
}

// Stop cancels tasks, drains the scheduler and pool, then closes task resources.
// Caller deadlines limit waiting; cleanup has its own bounded context. Failed
// resources remain owned by the manager and a later Stop retries their cleanup.
func (m *TaskManager) Stop(ctx context.Context) error {
	m.mu.Lock()
	if !m.stopping {
		m.stopping = true
		if m.cancel != nil {
			m.cancel()
		}
		attempt := &stopAttempt{done: make(chan struct{})}
		m.stop = attempt
		scheduled := m.scheduler.Stop()
		go func() {
			<-scheduled.Done()
			m.active.Wait()
			m.pool.Release()
			m.finishStop(ctx, attempt)
		}()
	} else {
		select {
		case <-m.stop.done:
			if m.stop.err != nil && len(m.tasks) > 0 {
				m.stop = &stopAttempt{done: make(chan struct{})}
				go m.finishStop(ctx, m.stop)
			}
		default:
		}
	}
	attempt := m.stop
	m.mu.Unlock()
	select {
	case <-attempt.done:
		return attempt.err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (m *TaskManager) finishStop(ctx context.Context, attempt *stopAttempt) {
	err := m.closeTasks(ctx)
	m.reportError(err)
	attempt.err = err
	close(attempt.done)
}

func (m *TaskManager) closeTasks(ctx context.Context) error {
	m.updateMu.Lock()
	defer m.updateMu.Unlock()
	// Begin the cleanup deadline after active work has drained. An expired
	// caller context must not prevent the manager from releasing its resources.
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), taskCleanupTimeout)
	defer cancel()
	m.mu.RLock()
	tasks := make([]*managedTask, 0, len(m.tasks))
	for _, task := range m.tasks {
		tasks = append(tasks, task)
	}
	m.mu.RUnlock()
	var errs []error
	for _, task := range tasks {
		if err := m.remove(cleanupCtx, task); err != nil {
			errs = append(errs, fmt.Errorf("stop task %s: %w", task.name, err))
		}
	}
	return errors.Join(errs...)
}

func (m *TaskManager) Exec(ctx context.Context) error {
	err := m.exec(ctx)
	m.reportError(err)
	return err
}

func (m *TaskManager) exec(ctx context.Context) error {
	m.execMu.Lock()
	defer m.execMu.Unlock()
	m.mu.RLock()
	stopping, started := m.stopping, m.started
	handlers := append([]SyncFunc(nil), m.syncHandlers...)
	m.mu.RUnlock()
	if stopping || !started {
		return errors.New("task manager is not running")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	var errs []error
	if m.autoSync && len(handlers) > 0 {
		synced, err := collectSyncedTasks(ctx, handlers)
		if err := m.syncTasks(ctx, synced, err); err != nil {
			errs = append(errs, err)
		}
	}
	m.mu.RLock()
	tasks := make([]*managedTask, 0, len(m.tasks))
	for _, task := range m.tasks {
		tasks = append(tasks, task)
	}
	m.mu.RUnlock()
	for _, task := range tasks {
		m.mu.Lock()
		if m.stopping {
			m.mu.Unlock()
			errs = append(errs, errors.New("task manager is stopping"))
			break
		}
		m.active.Add(1)
		m.mu.Unlock()
		// Task.Start may launch background work. Its context belongs to the
		// manager's running lifecycle, not to this scheduling invocation.
		if err := m.pool.Invoke(task); err != nil {
			m.active.Done()
			errs = append(errs, fmt.Errorf("submit task %s: %w", task.name, err))
		}
	}
	return errors.Join(errs...)
}

func collectSyncedTasks(ctx context.Context, handlers []SyncFunc) (map[string]*PeriodTask, error) {
	synced := make(map[string]*PeriodTask)
	for _, handler := range handlers {
		tasks, err := handler(ctx)
		if err != nil {
			return nil, err
		}
		for name, task := range tasks {
			if _, exists := synced[name]; exists {
				return nil, fmt.Errorf("multiple sync handlers returned task %s", name)
			}
			synced[name] = task
		}
	}
	return synced, nil
}

type PeriodTask struct {
	mu           sync.Mutex
	task         Task
	times        []RunPeriod
	updatePolicy UpdatePolicy
	enabled      bool
	status       TaskStatus
	closed       bool
	initFailed   bool
	skippedStop  bool
	cancel       context.CancelFunc
}

type RunPeriod struct {
	Start *time.Time
	End   *time.Time
}

func (p *RunPeriod) IsValid(target ...time.Time) bool {
	if p.Start == nil || p.End == nil {
		return false
	}
	now := time.Now()
	if len(target) > 0 {
		now = target[0]
	}
	start, end := p.Start.In(now.Location()), p.End.In(now.Location())
	start = time.Date(
		now.Year(),
		now.Month(),
		now.Day(),
		start.Hour(),
		start.Minute(),
		start.Second(),
		start.Nanosecond(),
		now.Location(),
	)
	end = time.Date(
		now.Year(),
		now.Month(),
		now.Day(),
		end.Hour(),
		end.Minute(),
		end.Second(),
		end.Nanosecond(),
		now.Location(),
	)
	if end.Before(start) {
		return !now.Before(start) || !now.After(end)
	}
	return !now.Before(start) && !now.After(end)
}

func (task *PeriodTask) timeValid(target ...time.Time) bool {
	for _, period := range task.times {
		if period.IsValid(target...) {
			return true
		}
	}
	return len(task.times) == 0
}

func (task *PeriodTask) TimeValid(target ...time.Time) bool {
	task.mu.Lock()
	defer task.mu.Unlock()
	return task.timeValid(target...)
}

func (task *PeriodTask) Status() TaskStatus {
	task.mu.Lock()
	defer task.mu.Unlock()
	return task.status
}

func (task *PeriodTask) SetEnabled(enabled bool) {
	task.mu.Lock()
	defer task.mu.Unlock()
	task.enabled = enabled
}

func (task *PeriodTask) Exec(ctx context.Context) error {
	task.mu.Lock()
	defer task.mu.Unlock()
	if task.closed || task.initFailed {
		return nil
	}
	if task.enabled && task.timeValid() {
		if err := ctx.Err(); err != nil {
			return err
		}
		if task.status == TaskStatusRunning {
			return nil
		}
		workerCtx, cancel := context.WithCancel(ctx)
		task.status = TaskStatusFailed
		defer func() {
			if task.status != TaskStatusRunning {
				cancel()
			}
		}()
		if err := task.task.Start(workerCtx); err != nil {
			return err
		}
		task.cancel = cancel
		task.status = TaskStatusRunning
		return nil
	}
	if task.status == TaskStatusInit {
		// The window never started this task. Removal still owns Init's resources.
		task.skippedStop = true
		task.status = TaskStatusStopped
	}
	if task.status == TaskStatusStopped {
		return nil
	}
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), taskCleanupTimeout)
	defer cancel()
	return task.stop(cleanupCtx)
}

func (task *PeriodTask) init(ctx context.Context) error {
	err := task.task.Init(ctx)
	task.mu.Lock()
	defer task.mu.Unlock()
	if err != nil {
		task.initFailed = true
		task.status = TaskStatusFailed
		return err
	}
	if task.initFailed {
		task.status = TaskStatusInit
		task.initFailed = false
	}
	return nil
}

func (task *PeriodTask) cleanupFailedInit(ctx context.Context) error {
	task.mu.Lock()
	defer task.mu.Unlock()
	return task.stop(ctx)
}

func (task *PeriodTask) close(ctx context.Context) error {
	task.mu.Lock()
	defer task.mu.Unlock()
	if task.closed {
		return nil
	}
	if task.status != TaskStatusStopped || task.skippedStop {
		if err := task.stop(ctx); err != nil {
			return err
		}
	}
	task.closed = true
	return nil
}

// stop requires task.mu. Failed initialization remains failed after cleanup.
func (task *PeriodTask) stop(ctx context.Context) error {
	if task.cancel != nil {
		task.cancel()
		task.cancel = nil
	}
	task.status = TaskStatusFailed
	if err := task.task.Stop(ctx); err != nil {
		return err
	}
	if !task.initFailed {
		task.status = TaskStatusStopped
	}
	task.skippedStop = false
	return nil
}

func (task *PeriodTask) updateSchedule(other *PeriodTask) {
	other.mu.Lock()
	times, enabled := append([]RunPeriod(nil), other.times...), other.enabled
	other.mu.Unlock()
	task.mu.Lock()
	task.times, task.enabled = times, enabled
	task.mu.Unlock()
}

func NewPeriodTask(task Task, times []RunPeriod, updatePolicy UpdatePolicy, enabled bool) *PeriodTask {
	if updatePolicy == "" {
		updatePolicy = AutoUpdate
	}
	periods := append([]RunPeriod(nil), times...)
	for index := range periods {
		if periods[index].Start != nil {
			value := *periods[index].Start
			periods[index].Start = &value
		}
		if periods[index].End != nil {
			value := *periods[index].End
			periods[index].End = &value
		}
	}
	return &PeriodTask{
		task:         task,
		times:        periods,
		updatePolicy: updatePolicy,
		enabled:      enabled,
		status:       TaskStatusInit,
	}
}
