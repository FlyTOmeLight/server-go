package modbus

import (
	"context"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"strconv"
	"testing"
	"time"

	"github.com/tbrandon/mbserver"
)

type registerFunc func(context.Context, uint8, int) (any, error)

func (f registerFunc) GetRegister(ctx context.Context, slaveID uint8, register int) (any, error) {
	return f(ctx, slaveID, register)
}

func TestStartReturnsBindFailure(t *testing.T) {
	reserved, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer reserved.Close()
	host, portString, _ := net.SplitHostPort(reserved.Addr().String())
	port, _ := strconv.Atoi(portString)
	service := NewDefaultModbusService(host, port, registerFunc(func(context.Context, uint8, int) (any, error) { return uint16(42), nil }))
	if err := service.Start(context.Background()); err == nil {
		t.Fatal("expected a bind error")
	}
	if service.ListenAddress() != "" {
		t.Fatal("failed startup retained a listener")
	}
	if err := service.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestServiceHandlesFragmentedTCPFramesAndStopsConnections(t *testing.T) {
	type contextKey struct{}
	ctx := context.WithValue(context.Background(), contextKey{}, "inherited")
	service := NewDefaultModbusService("127.0.0.1", 0, registerFunc(func(ctx context.Context, slaveID uint8, register int) (any, error) {
		if ctx.Value(contextKey{}) != "inherited" || slaveID != 1 || register != 10 {
			t.Errorf("register request lost context or addressing: slave=%d register=%d", slaveID, register)
		}
		return uint16(42), nil
	}))
	if err := service.Start(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = service.Stop(context.Background()) })
	connection, err := net.Dial("tcp", service.ListenAddress())
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	packet := readRequest(10).Bytes()
	if _, err := connection.Write(packet[:5]); err != nil {
		t.Fatal(err)
	}
	_ = connection.SetReadDeadline(time.Now().Add(30 * time.Millisecond))
	if _, err := connection.Read(make([]byte, 1)); err == nil {
		t.Fatal("partial TCP frame unexpectedly produced a response")
	} else if networkErr, ok := err.(net.Error); !ok || !networkErr.Timeout() {
		t.Fatalf("expected read timeout for partial frame, got %v", err)
	}
	_ = connection.SetReadDeadline(time.Now().Add(time.Second))
	if _, err := connection.Write(packet[5:8]); err != nil {
		t.Fatal(err)
	}
	if _, err := connection.Write(packet[8:]); err != nil {
		t.Fatal(err)
	}
	response := readResponse(t, connection)
	if response.GetFunction() != 3 || string(response.GetData()) != string([]byte{2, 0, 42}) {
		t.Fatalf("wrong response: %v", response)
	}
	stopCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := service.Stop(stopCtx); err != nil {
		t.Fatal(err)
	}
	if _, err := connection.Read(make([]byte, 1)); err == nil {
		t.Fatal("Stop left the accepted TCP connection open")
	}
	if err := service.Wait(stopCtx); err != nil {
		t.Fatal(err)
	}
}

func TestStopCancelsRegisterAPIAndRetainsCompletionAfterTimeout(t *testing.T) {
	started, canceled, release := make(chan struct{}), make(chan struct{}), make(chan struct{})
	service := NewDefaultModbusService("127.0.0.1", 0, registerFunc(func(ctx context.Context, _ uint8, _ int) (any, error) {
		close(started)
		<-ctx.Done()
		close(canceled)
		// Simulate cleanup that outlives the caller's first Stop deadline.
		<-release
		return nil, ctx.Err()
	}))
	if err := service.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		select {
		case <-release:
		default:
			close(release)
		}
		_ = service.Stop(context.Background())
	})
	connection, err := net.Dial("tcp", service.ListenAddress())
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	if _, err := connection.Write(readRequest(0).Bytes()); err != nil {
		t.Fatal(err)
	}
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("register request did not start")
	}
	stopCtx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := service.Stop(stopCtx); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected canceled first Stop, got %v", err)
	}
	select {
	case <-canceled:
	case <-time.After(time.Second):
		t.Fatal("Stop did not cancel register API")
	}
	close(release)
	stopCtx, cancel = context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := service.Stop(stopCtx); err != nil {
		t.Fatalf("second Stop lost worker completion: %v", err)
	}
}

func TestRegisterReadValidationAndBoolEncoding(t *testing.T) {
	service := NewDefaultModbusService("127.0.0.1", 0, registerFunc(func(context.Context, uint8, int) (any, error) { return true, nil }))
	if err := service.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = service.Stop(context.Background()) })
	connection, err := net.Dial("tcp", service.ListenAddress())
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	_ = connection.SetDeadline(time.Now().Add(time.Second))
	if _, err := connection.Write(readRequest(0).Bytes()); err != nil {
		t.Fatal(err)
	}
	if response := readResponse(t, connection); string(response.GetData()) != string([]byte{2, 0, 1}) {
		t.Fatalf("true did not encode as one 16-bit register: %v", response)
	}
	invalid := readRequest(0)
	invalid.SetData([]byte{0})
	if _, err := connection.Write(invalid.Bytes()); err != nil {
		t.Fatal(err)
	}
	if response := readResponse(t, connection); mbserver.GetException(response) != mbserver.IllegalDataValue {
		t.Fatalf("malformed read did not return Modbus error: %v", response)
	}
}

func readRequest(register uint16) *mbserver.TCPFrame {
	frame := &mbserver.TCPFrame{TransactionIdentifier: 1, Device: 1, Function: 3}
	mbserver.SetDataWithRegisterAndNumber(frame, register, 1)
	return frame
}

func readResponse(t *testing.T, connection net.Conn) *mbserver.TCPFrame {
	t.Helper()
	header := make([]byte, 6)
	if _, err := io.ReadFull(connection, header); err != nil {
		t.Fatal(err)
	}
	body := make([]byte, int(binary.BigEndian.Uint16(header[4:6])))
	if _, err := io.ReadFull(connection, body); err != nil {
		t.Fatal(err)
	}
	frame, err := mbserver.NewTCPFrame(append(header, body...))
	if err != nil {
		t.Fatal(err)
	}
	return frame
}
