package modbus

import "context"

// API reads registers. Implementations must stop promptly when ctx is canceled.
type API interface {
	GetRegister(ctx context.Context, slaveID uint8, regAddr int) (any, error)
}
