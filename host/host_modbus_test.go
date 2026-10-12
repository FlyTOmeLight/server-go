package host

import (
	"context"
	"io"
	"net"
	"strconv"
	"testing"
	"time"

	"github.com/FlyTOmeLight/server-go/pkg/modbus"
	"github.com/tbrandon/mbserver"
)

type modbusRegisterFunc func(context.Context, uint8, int) (any, error)

func (f modbusRegisterFunc) GetRegister(ctx context.Context, slaveID uint8, register int) (any, error) {
	return f(ctx, slaveID, register)
}

func TestModbusHostCancellationWaitsForAPIAndClosesConnection(t *testing.T) {
	called, stopped := make(chan struct{}), make(chan struct{})
	service := modbus.NewDefaultModbusService("127.0.0.1", 0, modbusRegisterFunc(func(ctx context.Context, _ uint8, _ int) (any, error) {
		close(called)
		<-ctx.Done()
		close(stopped)
		return nil, ctx.Err()
	}))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	result := make(chan error, 1)
	go func() { result <- NewModbusHost(service).Run(ctx) }()
	deadline := time.NewTimer(time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()
	for service.ListenAddress() == "" {
		select {
		case <-deadline.C:
			t.Fatal("Modbus service did not bind")
		case err := <-result:
			t.Fatalf("Modbus host failed: %v", err)
		case <-ticker.C:
		}
	}
	connection, err := net.Dial("tcp", service.ListenAddress())
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	frame := &mbserver.TCPFrame{TransactionIdentifier: 1, Device: 1, Function: 3}
	mbserver.SetDataWithRegisterAndNumber(frame, 0, 1)
	if _, err := connection.Write(frame.Bytes()); err != nil {
		t.Fatal(err)
	}
	select {
	case <-called:
	case <-time.After(time.Second):
		t.Fatal("register API was not called")
	}
	cancel()
	select {
	case err := <-result:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("Modbus host did not stop")
	}
	select {
	case <-stopped:
	default:
		t.Fatal("host returned before register API stopped")
	}
	_ = connection.SetReadDeadline(time.Now().Add(time.Second))
	// A response may already be buffered when cancellation closes the socket.
	if _, err := io.ReadAll(connection); err != nil {
		t.Fatalf("host cancellation did not close the accepted connection: %v", err)
	}
}

func TestModbusHostRollsBackWhenLaterServiceCannotBind(t *testing.T) {
	reserved, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer reserved.Close()
	_, portString, _ := net.SplitHostPort(reserved.Addr().String())
	port, _ := strconv.Atoi(portString)
	api := modbusRegisterFunc(func(context.Context, uint8, int) (any, error) { return uint16(1), nil })
	first := modbus.NewDefaultModbusService("127.0.0.1", 0, api)
	second := modbus.NewDefaultModbusService("127.0.0.1", port, api)
	if err := NewModbusHost(first, second).Run(context.Background()); err == nil {
		t.Fatal("expected later service bind failure")
	}
	connection, err := net.DialTimeout("tcp", first.ListenAddress(), time.Second)
	if err == nil {
		_ = connection.Close()
		t.Fatal("startup rollback left the first listener open")
	}
	if err := first.Wait(context.Background()); err != nil {
		t.Fatal(err)
	}
}
