package cron

import (
	"context"
	stderr "errors"
	"fmt"
	"sync"
	"time"

	"github.com/FlyTOmeLight/server-go/logger"
	"github.com/panjf2000/ants/v2"
	"github.com/robfig/cron/v3"

	"github.com/FlyTOmeLight/server-go/errors"
	core "github.com/FlyTOmeLight/server-go/pkg"
)

type Task interface {
	Name() string
	Init() error
	Start() error
	Stop()
}

const (
	TaskStatusInit    TaskStatus = "Init"
	TaskStatusRunning TaskStatus = "Running"
	TaskStatusStopped TaskStatus = "Stopped"
	TaskStatusFailed  TaskStatus = "Failed"
)

type TaskStatus string

const (
	AutoUpdate      UpdatePolicy = "Auto"
	ForbiddenUpdate UpdatePolicy = "Forbidden"
	Recreate        UpdatePolicy = "Recreate"
)

type UpdatePolicy string

type TaskManager struct {
	tasks     map[string]*PeriodTask
	scheduler *cron.Cron

	autoSync     bool
	syncHandlers []SyncFunc

	concurrency int

	pool   *ants.PoolWithFunc
	poolMu sync.Mutex
}

type SyncFunc func() (map[string]*PeriodTask, error)

func (m *TaskManager) RegisterSyncHandler(handler SyncFunc) {
	m.syncHandlers = append(m.syncHandlers, handler)
}

func (m *TaskManager) SyncTasks(tasks map[string]*PeriodTask, err error) {
	if err != nil {
		logger.Errorf(&core.Context{}, "SyncTasksError monitor sync task failed err: %v", err)
		return
	}

	if m.tasks == nil {
		m.tasks = make(map[string]*PeriodTask)
	}

	for name, task := range tasks {
		switch task.UpdatePolicy {
		case ForbiddenUpdate:
			if m.tasks[name] != nil {
				continue
			}
		}
		m.tasks[name] = task
	}

	for name := range m.tasks {
		if tasks[name] == nil {
			m.Remove(name)
		}
	}
}

func (m *TaskManager) Add(task *PeriodTask) error {
	if m.tasks == nil {
		m.tasks = make(map[string]*PeriodTask)
	}

	name := task.Task.Name()
	if name == "" {
		return stderr.New("task name is empty")
	}

	switch task.UpdatePolicy {
	case ForbiddenUpdate:
		if m.tasks[name] != nil {
			return stderr.New("task already exists")
		}
	case Recreate:
		if m.tasks[name] != nil {
			m.Remove(name)
		}
	}

	m.tasks[name] = task
	return task.Task.Init()
}

func (m *TaskManager) GetPeriodTask(name string) *PeriodTask {
	if m.tasks == nil {
		return nil
	}

	return m.tasks[name]
}

func (m *TaskManager) GetTask(name string) Task {
	if m.tasks == nil {
		return nil
	}

	pt := m.tasks[name]
	if pt == nil {
		return nil
	}

	return pt.Task
}

func (m *TaskManager) Remove(name string) {
	if m.tasks == nil {
		return
	}

	task := m.tasks[name]
	if task != nil {
		task.Task.Stop()
	}

	delete(m.tasks, name)
}

func (m *TaskManager) StartAsync() {
	if m == nil {
		return
	}
	m.scheduler.Start()
}

func (m *TaskManager) StopAsync() {
	if m == nil {
		return
	}
	m.scheduler.Stop()
	m.poolMu.Lock()
	if m.pool != nil {
		m.pool.Release()
		m.pool = nil
	}
	m.poolMu.Unlock()
}

func (m *TaskManager) Exec() {
	if m.autoSync {
		for _, handler := range m.syncHandlers {
			m.SyncTasks(handler())
		}
	}
	m.poolMu.Lock()
	defer m.poolMu.Unlock()

	ctx := context.Background()
	for _, task := range m.tasks {
		logger.Debugf(ctx, "monitor exec task name: %s, status: %s", task.Task.Name(), task.status)
		err := m.pool.Invoke(task)
		if err != nil {
			logger.Errorf(ctx, "monitor execute task failed err: %v", err)
			continue
		}
	}
}

func NewTaskManager(interval time.Duration, concurrency int, autoSync bool) (*TaskManager, error) {
	if concurrency == 0 {
		concurrency = 1
	}

	cronJob := cron.New(cron.WithSeconds())
	pool, err := ants.NewPoolWithFunc(concurrency, func(payload any) {
		if task, ok := payload.(*PeriodTask); ok {
			task.Exec()
		}
	})
	if err != nil {
		return nil, errors.Wrap(err, "initialize goroutine pool")
	}

	tm := &TaskManager{
		scheduler:    cronJob,
		tasks:        make(map[string]*PeriodTask),
		syncHandlers: []SyncFunc{},
		autoSync:     autoSync,
		concurrency:  concurrency,
		pool:         pool,
	}

	spec := fmt.Sprintf("*/%d * * * * ?", interval)
	_, err = cronJob.AddFunc(spec, func() {
		defer func() {
			if r := recover(); r != nil {
				fmt.Printf("Recovered from panic in task manager: %v\n", r)
			}
		}()
		tm.Exec()
	})
	if err != nil {
		return nil, errors.Wrap(err, "start task manager")
	}

	return tm, nil
}

type PeriodTask struct {
	Task         Task
	Times        []RunPeriod
	UpdatePolicy UpdatePolicy
	Enabled      bool
	status       TaskStatus
}

type RunPeriod struct {
	Start *time.Time
	End   *time.Time
}

func (p *RunPeriod) IsValid(target ...time.Time) bool {
	targetTime := time.Now()

	if len(target) > 0 {
		targetTime = target[0]
	}
	*p.Start = time.Date(targetTime.Year(), targetTime.Month(), targetTime.Day(),
		p.Start.Hour(), p.Start.Minute(), p.Start.Second(), p.Start.Nanosecond(), time.Local)

	*p.End = time.Date(targetTime.Year(), targetTime.Month(), targetTime.Day(),
		p.End.Hour(), p.End.Minute(), 59, p.End.Nanosecond(), time.Local)

	return p.Start.Before(targetTime) && p.End.After(targetTime)
}

func (task *PeriodTask) TimeValid(target ...time.Time) bool {
	for _, t := range task.Times {
		if t.IsValid(target...) {
			return true
		}
	}
	return false
}

func (task *PeriodTask) Exec() {
	timeValid := task.TimeValid()

	if task.Enabled && timeValid && task.status != TaskStatusRunning {
		task.status = TaskStatusRunning
		err := task.Task.Start()
		if err != nil {
			task.status = TaskStatusFailed
		}
		return
	}

	if (!timeValid || !task.Enabled) && task.status != TaskStatusStopped {
		task.status = TaskStatusStopped
		task.Task.Stop()
	}
}

func NewPeriodTask(task Task, times []RunPeriod, updatePolicy UpdatePolicy, enabled bool) *PeriodTask {
	if updatePolicy == "" {
		updatePolicy = AutoUpdate
	}

	return &PeriodTask{Task: task, status: TaskStatusInit, Times: times, UpdatePolicy: updatePolicy, Enabled: enabled}
}
