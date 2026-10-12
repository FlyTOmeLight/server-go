package pubsub

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	daprp "github.com/dapr/go-sdk/dapr/proto/runtime/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/stats"
	"google.golang.org/protobuf/types/known/emptypb"
)

type publishingServer struct {
	daprp.UnimplementedDaprServer
	requests chan *daprp.PublishEventRequest
}

func (s *publishingServer) PublishEvent(_ context.Context, request *daprp.PublishEventRequest) (*emptypb.Empty, error) {
	s.requests <- request
	return &emptypb.Empty{}, nil
}

type connectionStats struct{ closed chan struct{} }

func (s *connectionStats) TagRPC(ctx context.Context, _ *stats.RPCTagInfo) context.Context {
	return ctx
}
func (s *connectionStats) HandleRPC(context.Context, stats.RPCStats) {}
func (s *connectionStats) TagConn(ctx context.Context, _ *stats.ConnTagInfo) context.Context {
	return ctx
}
func (s *connectionStats) HandleConn(_ context.Context, event stats.ConnStats) {
	if _, closed := event.(*stats.ConnEnd); closed {
		s.closed <- struct{}{}
	}
}

func TestPublishEventUsesTopicAndClosesEachOwnedConnection(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	_, port, _ := net.SplitHostPort(listener.Addr().String())
	t.Setenv("DAPR_GRPC_PORT", port)
	t.Setenv("DAPR_CLIENT_TIMEOUT_SECONDS", "1")
	tracker := &connectionStats{closed: make(chan struct{}, 4)}
	server := grpc.NewServer(grpc.StatsHandler(tracker))
	publisher := &publishingServer{requests: make(chan *daprp.PublishEventRequest, 2)}
	daprp.RegisterDaprServer(server, publisher)
	served := make(chan error, 1)
	go func() { served <- server.Serve(listener) }()
	t.Cleanup(func() {
		server.Stop()
		_ = listener.Close()
		<-served
	})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	for range 2 {
		if err := PublishEvent(ctx, "component", "actual-topic", "payload"); err != nil {
			t.Fatal(err)
		}
		select {
		case request := <-publisher.requests:
			if request.PubsubName != "component" || request.Topic != "actual-topic" || string(request.Data) != "payload" {
				t.Fatalf("wrong published event: %v", request)
			}
		case <-ctx.Done():
			t.Fatal("event was not delivered")
		}
		select {
		case <-tracker.closed:
		case <-ctx.Done():
			t.Fatal("publish operation left its connection open")
		}
	}
	canceled, stop := context.WithCancel(context.Background())
	stop()
	if err := PublishEvent(canceled, "component", "actual-topic", "payload"); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected canceled publish, got %v", err)
	}
}
