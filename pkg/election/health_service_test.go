package election

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/FlyTOmeLight/server-go/conf"
	serrors "github.com/FlyTOmeLight/server-go/errors"
)

type standbyElection struct{ *DummyAppElection }

func (standbyElection) IsLeader() bool { return false }

func (standbyElection) GetLeader() string { return "another-instance" }

func TestLeaderHealthRoutesCanBeRegisteredOnFreshRouters(t *testing.T) {
	for _, test := range []struct {
		name   string
		e      LeaderElection
		status int
	}{
		{name: "leader", e: NewDummyAppElection("leader"), status: http.StatusOK},
		{name: "standby", e: standbyElection{NewDummyAppElection("standby")}, status: http.StatusServiceUnavailable},
	} {
		t.Run(test.name, func(t *testing.T) {
			service := NewLeaderHealthService(test.e)
			for range 2 {
				engine := gin.New()
				engine.Use(serrors.ErrorHandler(&conf.Config{AppCode: 123}))
				if err := service.RegisterRoutes(engine.Group("/v1")); err != nil {
					t.Fatal(err)
				}
				writer := httptest.NewRecorder()
				engine.ServeHTTP(
					writer,
					httptest.NewRequest(http.MethodGet, "/v1/election/isleader", nil),
				)
				if writer.Code != test.status {
					t.Fatalf("status=%d body=%s", writer.Code, writer.Body.String())
				}
			}
		})
	}
}

func TestDummyRunWaitsForTasksAfterCancellation(t *testing.T) {
	election := NewDummyAppElection("instance")
	started, canceled, release := make(chan struct{}), make(chan struct{}), make(chan struct{})
	var releaseOnce sync.Once
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	election.RunOnLeader(func(ctx context.Context) {
		close(started)
		<-ctx.Done()
		close(canceled)
		<-release
	})
	done := make(chan struct{})
	go func() { election.Run(ctx); close(done) }()
	t.Cleanup(func() {
		cancel()
		releaseOnce.Do(func() { close(release) })
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Error("dummy election leaked its run loop")
		}
	})
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("leader task did not start")
	}
	cancel()
	select {
	case <-canceled:
	case <-time.After(time.Second):
		t.Fatal("leader task did not inherit cancellation")
	}
	select {
	case <-done:
		t.Fatal("Run returned before its task completed")
	default:
	}
	releaseOnce.Do(func() { close(release) })
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Run did not wait for task completion")
	}
}
