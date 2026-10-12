package modbus

import (
	"context"
	"encoding/binary"
	"errors"
	"io"
	"math"
	"net"
	"strconv"
	"sync"

	"github.com/tbrandon/mbserver"
)

// DefaultModbusService owns its TCP listener and every accepted connection.
// It supports holding-register reads (function 3); other functions are rejected.
type DefaultModbusService struct {
	api         API
	address     string
	port        int
	mu          sync.Mutex
	processMu   sync.Mutex
	listener    net.Listener
	connections map[net.Conn]struct{}
	workers     sync.WaitGroup
	cancel      context.CancelFunc
	done        chan struct{}
	serveErr    error
	started     bool
	closed      bool
}

func NewDefaultModbusService(address string, port int, api API) *DefaultModbusService {
	return &DefaultModbusService{api: api, address: address, port: port, connections: make(map[net.Conn]struct{})}
}

func (svc *DefaultModbusService) ListenAddress() string {
	svc.mu.Lock()
	defer svc.mu.Unlock()
	if svc.listener == nil {
		return ""
	}
	return svc.listener.Addr().String()
}

// Start binds synchronously so startup failures are returned to the application.
func (svc *DefaultModbusService) Start(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	svc.mu.Lock()
	defer svc.mu.Unlock()
	if svc.started || svc.closed {
		return errors.New("Modbus service can only start once")
	}
	if svc.api == nil {
		return errors.New("Modbus register API is required")
	}
	listener, err := net.Listen("tcp", net.JoinHostPort(svc.address, strconv.Itoa(svc.port)))
	if err != nil {
		return err
	}
	runCtx, cancel := context.WithCancel(ctx)
	svc.listener, svc.cancel = listener, cancel
	svc.started = true
	svc.done = make(chan struct{})
	go svc.accept(runCtx, listener)
	return nil
}

// Wait reports an unexpected listener failure, or returns when ctx is canceled.
func (svc *DefaultModbusService) Wait(ctx context.Context) error {
	svc.mu.Lock()
	done := svc.done
	svc.mu.Unlock()
	if done == nil {
		return errors.New("Modbus service is not started")
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-done:
		svc.mu.Lock()
		defer svc.mu.Unlock()
		return svc.serveErr
	}
}

// Stop cancels register calls, closes active connections and waits for workers.
func (svc *DefaultModbusService) Stop(ctx context.Context) error {
	svc.shutdown()
	svc.mu.Lock()
	done := svc.done
	svc.mu.Unlock()
	if done == nil {
		return nil
	}
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (svc *DefaultModbusService) shutdown() {
	svc.mu.Lock()
	defer svc.mu.Unlock()
	svc.closed = true
	if svc.cancel != nil {
		svc.cancel()
	}
	if svc.listener != nil {
		_ = svc.listener.Close()
	}
	for connection := range svc.connections {
		_ = connection.Close()
	}
}

func (svc *DefaultModbusService) accept(ctx context.Context, listener net.Listener) {
	defer func() {
		svc.shutdown()
		svc.workers.Wait()
		close(svc.done)
	}()
	for {
		connection, err := listener.Accept()
		if err != nil {
			if !errors.Is(err, net.ErrClosed) {
				svc.mu.Lock()
				svc.serveErr = err
				svc.mu.Unlock()
			}
			return
		}
		svc.mu.Lock()
		if svc.closed {
			svc.mu.Unlock()
			_ = connection.Close()
			return
		}
		svc.connections[connection] = struct{}{}
		svc.workers.Add(1)
		svc.mu.Unlock()
		go svc.serveConnection(ctx, connection)
	}
}

func (svc *DefaultModbusService) serveConnection(ctx context.Context, connection net.Conn) {
	defer svc.workers.Done()
	defer func() {
		_ = connection.Close()
		svc.mu.Lock()
		delete(svc.connections, connection)
		svc.mu.Unlock()
	}()
	for {
		// A TCP read can contain part of a frame or several frames. Read exactly
		// the MBAP header and declared body before using mbserver's parser.
		header := make([]byte, 6)
		if _, err := io.ReadFull(connection, header); err != nil {
			return
		}
		body := make([]byte, int(binary.BigEndian.Uint16(header[4:6])))
		if _, err := io.ReadFull(connection, body); err != nil {
			return
		}
		frame, err := mbserver.NewTCPFrame(append(header, body...))
		if err != nil {
			return
		}
		svc.processMu.Lock()
		data, exception := svc.readFrame(ctx, frame)
		svc.processMu.Unlock()
		if exception != mbserver.Success {
			frame.SetException(&exception)
		} else {
			frame.SetData(data)
		}
		if _, err := connection.Write(frame.Bytes()); err != nil {
			return
		}
	}
}

func (svc *DefaultModbusService) readFrame(ctx context.Context, frame *mbserver.TCPFrame) ([]byte, mbserver.Exception) {
	if frame.GetFunction() != 3 {
		return nil, mbserver.IllegalFunction
	}
	if len(frame.GetData()) != 4 {
		return nil, mbserver.IllegalDataValue
	}
	register := int(binary.BigEndian.Uint16(frame.GetData()[:2]))
	numRegs := int(binary.BigEndian.Uint16(frame.GetData()[2:]))
	if numRegs == 0 || numRegs > 125 {
		return nil, mbserver.IllegalDataValue
	}
	if register+numRegs > 65536 {
		return nil, mbserver.IllegalDataAddress
	}
	data := make([]byte, numRegs*2)
	for offset := 0; offset < len(data); {
		if ctx.Err() != nil {
			return nil, mbserver.SlaveDeviceFailure
		}
		value, size, exception := svc.readDBData(ctx, frame.Device, register+offset/2)
		if exception != mbserver.Success {
			return nil, exception
		}
		copy(data[offset:], value[:size])
		offset += size
	}
	return append([]byte{byte(len(data))}, data...), mbserver.Success
}

// 读取数据，并根据数据类型写入寄存器
func (svc *DefaultModbusService) readDBData(
	ctx context.Context,
	slaveID uint8,
	regAddr int,
) ([]byte, int, mbserver.Exception) {
	regValue, err := svc.api.GetRegister(ctx, slaveID, regAddr)
	bytes := make([]byte, 8)
	if err != nil {
		return bytes, 0, mbserver.IllegalDataAddress
	}

	boolValue, ok := regValue.(bool)
	if ok {
		if boolValue {
			binary.BigEndian.PutUint16(bytes, 1)
		}
		return bytes, 2, mbserver.Success
	}

	int16Value, ok := regValue.(int16)
	if ok {
		binary.BigEndian.PutUint16(bytes, uint16(int16Value))
		return bytes, 2, mbserver.Success
	}

	uint16Value, ok := regValue.(uint16)
	if ok {
		binary.BigEndian.PutUint16(bytes, uint16Value)
		return bytes, 2, mbserver.Success
	}

	int32Value, ok := regValue.(int32)
	if ok {
		binary.BigEndian.PutUint32(bytes, uint32(int32Value))
		return bytes, 4, mbserver.Success
	}

	uint32Value, ok := regValue.(uint32)
	if ok {
		binary.BigEndian.PutUint32(bytes, uint32Value)
		return bytes, 4, mbserver.Success
	}

	float32Value, ok := regValue.(float32)
	if ok {
		frameValue1 := float32(float32Value)
		binary.BigEndian.PutUint32(bytes, math.Float32bits(frameValue1))
		return bytes, 4, mbserver.Success
	}

	int64Value, ok := regValue.(int64)
	if ok {
		binary.BigEndian.PutUint64(bytes, uint64(int64Value))
		return bytes, 8, mbserver.Success
	}

	uint64Value, ok := regValue.(uint64)
	if ok {
		binary.BigEndian.PutUint64(bytes, uint64Value)
		return bytes, 8, mbserver.Success
	}

	float64Value, ok := regValue.(float64)
	if ok {
		binary.BigEndian.PutUint64(bytes, math.Float64bits(float64Value))
		return bytes, 8, mbserver.Success
	}
	return bytes, 0, mbserver.IllegalDataValue
}
