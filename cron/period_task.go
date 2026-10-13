package cron

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/panjf2000/ants/v2"
	"github.com/robfig/cron/v3"
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
	tasks        map[string]*PeriodTask
	scheduler    *cron.Cron
	pool         *ants.PoolWithFunc
	autoSync     bool
	syncHandlers []SyncFunc
	active       sync.WaitGroup
	started      bool
	stopping     bool
	runCtx       context.Context
	cancel       context.CancelFunc
	stop         *stopAttempt
}

type execution struct {
	ctx  context.Context
	task *PeriodTask
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
		tasks: make(map[string]*PeriodTask), scheduler: cron.New(),
		autoSync: autoSync,
	}
	pool, err := ants.NewPoolWithFunc(concurrency, func(payload any) {
		defer manager.active.Done()
		job := payload.(execution)
		_ = job.task.Exec(job.ctx)
	})
	if err != nil {
		return nil, fmt.Errorf("initialize task pool: %w", err)
	}
	manager.pool = pool
	manager.scheduler.Schedule(cron.Every(interval), cron.FuncJob(func() {
		_ = manager.Exec(manager.runCtx)
	}))
	return manager, nil
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
	if task == nil || task.task == nil || task.task.Name() == "" {
		return errors.New("task and its name must be provided")
	}
	switch task.updatePolicy {
	case AutoUpdate, ForbiddenUpdate, Recreate:
	default:
		return fmt.Errorf("unknown task update policy: %s", task.updatePolicy)
	}
	m.updateMu.Lock()
	defer m.updateMu.Unlock()
	name := task.task.Name()
	m.mu.RLock()
	old, stopping := m.tasks[name], m.stopping
	m.mu.RUnlock()
	if stopping {
		return errors.New("task manager is stopping")
	}
	if old != nil {
		switch task.updatePolicy {
		case ForbiddenUpdate:
			return errors.New("task already exists")
		case AutoUpdate:
			old.updateSchedule(task)
			return nil
		case Recreate:
			if err := old.close(ctx); err != nil {
				return err
			}
			m.mu.Lock()
			delete(m.tasks, name)
			m.mu.Unlock()
		default:
			return fmt.Errorf("unknown task update policy: %s", task.updatePolicy)
		}
	}
	if err := task.task.Init(ctx); err != nil {
		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), taskCleanupTimeout)
		cleanupErr := task.close(cleanupCtx)
		cancel()
		if cleanupErr != nil {
			// Failed initialization still owns resources when its cleanup fails.
			m.mu.Lock()
			m.tasks[name] = task
			m.mu.Unlock()
		}
		task.mu.Lock()
		task.status = TaskStatusFailed
		task.enabled = false
		task.mu.Unlock()
		return errors.Join(err, cleanupErr)
	}
	m.mu.Lock()
	m.tasks[name] = task
	m.mu.Unlock()
	return nil
}

func (m *TaskManager) SyncTasks(ctx context.Context, tasks map[string]*PeriodTask, syncErr error) error {
	if syncErr != nil {
		return syncErr
	}
	for name, task := range tasks {
		if task == nil || task.task == nil || task.task.Name() != name {
			return fmt.Errorf("invalid synced task: %s", name)
		}
		if task.updatePolicy == ForbiddenUpdate && m.GetPeriodTask(name) != nil {
			continue
		}
		if err := m.Add(ctx, task); err != nil {
			return err
		}
	}
	m.mu.RLock()
	var deleted []string
	for name := range m.tasks {
		if tasks[name] == nil {
			deleted = append(deleted, name)
		}
	}
	m.mu.RUnlock()
	var errs []error
	for _, name := range deleted {
		errs = append(errs, m.Remove(ctx, name))
	}
	return errors.Join(errs...)
}

func (m *TaskManager) GetPeriodTask(name string) *PeriodTask {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.tasks[name]
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
	if err := task.close(ctx); err != nil {
		return err
	}
	m.mu.Lock()
	delete(m.tasks, name)
	m.mu.Unlock()
	return nil
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
	m.updateMu.Lock()
	defer m.updateMu.Unlock()
	// Begin the cleanup deadline after active work has drained. An expired
	// caller context must not prevent the manager from releasing its resources.
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), taskCleanupTimeout)
	defer cancel()
	m.mu.RLock()
	tasks := make(map[string]*PeriodTask, len(m.tasks))
	for name, task := range m.tasks {
		tasks[name] = task
	}
	m.mu.RUnlock()
	var errs []error
	for name, task := range tasks {
		if err := task.close(cleanupCtx); err != nil {
			errs = append(errs, fmt.Errorf("stop task %s: %w", name, err))
			continue
		}
		m.mu.Lock()
		delete(m.tasks, name)
		m.mu.Unlock()
	}
	attempt.err = errors.Join(errs...)
	close(attempt.done)
}

func (m *TaskManager) Exec(ctx context.Context) error {
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
	if m.autoSync {
		synced := make(map[string]*PeriodTask)
		for _, handler := range handlers {
			tasks, err := handler(ctx)
			if err != nil {
				return err
			}
			for name, task := range tasks {
				if _, exists := synced[name]; exists {
					return fmt.Errorf("multiple sync handlers returned task %s", name)
				}
				synced[name] = task
			}
		}
		if len(handlers) > 0 {
			if err := m.SyncTasks(ctx, synced, nil); err != nil {
				return err
			}
		}
	}
	m.mu.RLock()
	tasks := make([]*PeriodTask, 0, len(m.tasks))
	for _, task := range m.tasks {
		tasks = append(tasks, task)
	}
	m.mu.RUnlock()
	for _, task := range tasks {
		m.mu.Lock()
		if m.stopping {
			m.mu.Unlock()
			return errors.New("task manager is stopping")
		}
		m.active.Add(1)
		m.mu.Unlock()
		// Task.Start may launch background work. Its context belongs to the
		// manager's running lifecycle, not to this scheduling invocation.
		if err := m.pool.Invoke(execution{ctx: m.runCtx, task: task}); err != nil {
			m.active.Done()
			return err
		}
	}
	return nil
}

type PeriodTask struct {
	mu           sync.Mutex
	task         Task
	times        []RunPeriod
	updatePolicy UpdatePolicy
	enabled      bool
	status       TaskStatus
	closed       bool
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
	start := time.Date(
		now.Year(),
		now.Month(),
		now.Day(),
		p.Start.Hour(),
		p.Start.Minute(),
		p.Start.Second(),
		p.Start.Nanosecond(),
		now.Location(),
	)
	end := time.Date(
		now.Year(),
		now.Month(),
		now.Day(),
		p.End.Hour(),
		p.End.Minute(),
		p.End.Second(),
		p.End.Nanosecond(),
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
	if task.closed {
		return nil
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if task.enabled && task.timeValid() {
		if task.status == TaskStatusRunning {
			return nil
		}
		workerCtx, cancel := context.WithCancel(ctx)
		if err := task.task.Start(workerCtx); err != nil {
			cancel()
			task.status = TaskStatusFailed
			return err
		}
		task.cancel = cancel
		task.status = TaskStatusRunning
		return nil
	}
	if task.status != TaskStatusStopped {
		if task.cancel != nil {
			task.cancel()
			task.cancel = nil
		}
		if err := task.task.Stop(ctx); err != nil {
			task.status = TaskStatusFailed
			return err
		}
		task.status = TaskStatusStopped
	}
	return nil
}

func (task *PeriodTask) close(ctx context.Context) error {
	task.mu.Lock()
	defer task.mu.Unlock()
	if task.closed {
		return nil
	}
	if task.status != TaskStatusStopped {
		if task.cancel != nil {
			task.cancel()
			task.cancel = nil
		}
		if err := task.task.Stop(ctx); err != nil {
			task.status = TaskStatusFailed
			return err
		}
	}
	task.status = TaskStatusStopped
	task.closed = true
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
