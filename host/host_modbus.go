package host

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/FlyTOmeLight/server-go/pkg/modbus"
)

// ModbusHost owns the lifecycle of its service instances. The application owns
// process signals and supplies their cancellation through Run's context.
type ModbusHost struct {
	services []*modbus.DefaultModbusService
	mu       sync.Mutex
	run      bool
}

func NewModbusHost(services ...*modbus.DefaultModbusService) *ModbusHost {
	return &ModbusHost{services: append([]*modbus.DefaultModbusService(nil), services...)}
}

func (h *ModbusHost) Run(ctx context.Context) (result error) {
	if err := ctx.Err(); err != nil {
		return err
	}
	h.mu.Lock()
	if h.run {
		h.mu.Unlock()
		return errors.New("Modbus host can only run once")
	}
	h.run = true
	h.mu.Unlock()
	if len(h.services) == 0 {
		return errors.New("Modbus host requires a service")
	}

	workerCtx, cancel := context.WithCancel(ctx)
	started := make([]*modbus.DefaultModbusService, 0, len(h.services))
	var waiters sync.WaitGroup
	defer func() {
		cancel()
		stopCtx, stopCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer stopCancel()
		for i := len(started) - 1; i >= 0; i-- {
			result = errors.Join(result, started[i].Stop(stopCtx))
		}
		waiters.Wait()
	}()
	for _, service := range h.services {
		if service == nil {
			return errors.New("Modbus service is nil")
		}
		if err := service.Start(workerCtx); err != nil {
			return fmt.Errorf("start Modbus service: %w", err)
		}
		started = append(started, service)
	}
	stopped := make(chan error, len(started))
	for _, service := range started {
		waiters.Add(1)
		go func() {
			defer waiters.Done()
			stopped <- service.Wait(workerCtx)
		}()
	}
	select {
	case <-ctx.Done():
		return nil
	case err := <-stopped:
		if ctx.Err() != nil {
			return nil
		}
		return err
	}
}
