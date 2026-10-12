package host

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/net/http2"

	"github.com/FlyTOmeLight/server-go/conf"
)

func TestShutdownTimeoutJoinsHandlersAndGivesStopItsOwnBudget(t *testing.T) {
	for _, h2c := range []bool{false, true} {
		name := "HTTP1"
		if h2c {
			name = "H2C"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			entered, canceled, release := make(chan struct{}), make(chan struct{}), make(chan struct{})
			var releaseOnce sync.Once
			var finished atomic.Bool
			stopResult := make(chan error, 1)
			svc := &testService{handle: func(c *gin.Context) {
				close(entered)
				<-c.Request.Context().Done()
				close(canceled)
				// Resource cleanup can continue after network cancellation.
				<-release
				finished.Store(true)
			}}
			svc.stop = func(ctx context.Context) error {
				var err error
				if !finished.Load() {
					err = errors.New("service stopped before handler finished")
				} else if ctx.Err() != nil {
					err = errors.New("HTTP drain consumed service cleanup budget")
				} else if deadline, ok := ctx.Deadline(); !ok || time.Until(deadline) < 4*time.Second {
					err = errors.New("service did not receive its own cleanup deadline")
				}
				stopResult <- err
				return err
			}
			h, _ := newTestHostWithHTTPConfig(t, svc, conf.HTTPServerConfig{UseH2C: h2c})
			cancel, done := startTestHost(t, h)
			t.Cleanup(func() {
				cancel()
				releaseOnce.Do(func() { close(release) })
			})
			client := shutdownClient(t, h2c)
			clientDone := make(chan error, 1)
			go func() {
				response, err := client.Get("http://" + h.Address() + "/business")
				if err == nil {
					_, err = io.Copy(io.Discard, response.Body)
					_ = response.Body.Close()
				}
				clientDone <- err
			}()
			select {
			case <-entered:
			case <-time.After(3 * time.Second):
				t.Fatal("request did not enter")
			}
			cancel()
			waitDraining(t, h)
			writer := httptest.NewRecorder()
			h.Handler().ServeHTTP(writer, httptest.NewRequest(http.MethodGet, "/business", nil))
			if writer.Code != http.StatusServiceUnavailable {
				t.Fatal("host admitted new work during shutdown")
			}
			select {
			case <-canceled:
			case <-time.After(8 * time.Second):
				t.Fatal("force shutdown did not cancel the request")
			}
			select {
			case err := <-done:
				t.Fatalf("Run returned while handler still used resources: %v", err)
			case <-stopResult:
				t.Fatal("Stop ran while handler still used resources")
			default:
			}
			select {
			case err := <-clientDone:
				if err == nil {
					t.Fatal("force shutdown did not close the active connection")
				}
			case <-time.After(time.Second):
				t.Fatal("active client connection remained open")
			}
			releaseOnce.Do(func() { close(release) })
			select {
			case err := <-done:
				if !errors.Is(err, context.DeadlineExceeded) {
					t.Fatalf("expected drain deadline error, got %v", err)
				}
			case <-time.After(3 * time.Second):
				t.Fatal("host did not wait for handler completion")
			}
			if err := <-stopResult; err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestH2CShutdownDrainsResponseAndClosesIdleConnection(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	svc := &testService{handle: func(c *gin.Context) {
		close(entered)
		<-release
		c.String(http.StatusOK, "complete")
	}}
	h, _ := newTestHostWithHTTPConfig(t, svc, conf.HTTPServerConfig{UseH2C: true})
	cancel, done := startTestHost(t, h)
	defer cancel()
	connectionClosed := make(chan struct{}, 1)
	client := shutdownClientObserved(t, true, func() { connectionClosed <- struct{}{} })
	responseDone := make(chan error, 1)
	go func() {
		response, err := client.Get("http://" + h.Address() + "/business")
		if err == nil {
			body, readErr := io.ReadAll(response.Body)
			_ = response.Body.Close()
			if response.ProtoMajor != 2 || string(body) != "complete" {
				err = errors.New("H2C response was not drained")
			} else {
				err = readErr
			}
		}
		responseDone <- err
	}()
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("H2C request did not enter")
	}
	cancel()
	waitDraining(t, h)
	close(release)
	select {
	case err := <-responseDone:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("H2C response did not drain")
	}
	waitStopped(t, done)
	select {
	case <-connectionClosed:
	case <-time.After(time.Second):
		t.Fatal("H2C shutdown left the client connection open")
	}
	if svc.stops.Load() != 1 {
		t.Fatal("H2C shutdown did not close the service once")
	}
}

func shutdownClient(t *testing.T, h2c bool) *http.Client {
	t.Helper()
	return shutdownClientObserved(t, h2c, nil)
}

func shutdownClientObserved(t *testing.T, h2c bool, onClose func()) *http.Client {
	t.Helper()
	if h2c {
		transport := &http2.Transport{
			AllowHTTP: true,
			DialTLSContext: func(ctx context.Context, network, address string, _ *tls.Config) (net.Conn, error) {
				connection, err := (&net.Dialer{}).DialContext(ctx, network, address)
				if err == nil && onClose != nil {
					connection = &observedClientConnection{Conn: connection, onClose: onClose}
				}
				return connection, err
			},
		}
		t.Cleanup(transport.CloseIdleConnections)
		return &http.Client{Transport: transport}
	}
	transport := &http.Transport{DisableKeepAlives: true}
	t.Cleanup(transport.CloseIdleConnections)
	return &http.Client{Transport: transport}
}

type observedClientConnection struct {
	net.Conn
	onClose func()
	once    sync.Once
}

func (c *observedClientConnection) Close() error {
	err := c.Conn.Close()
	c.once.Do(c.onClose)
	return err
}

func waitDraining(t *testing.T, h *HTTPHost) {
	t.Helper()
	deadline := time.NewTimer(time.Second)
	defer deadline.Stop()
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()
	for {
		h.requestMu.Lock()
		draining := h.draining
		h.requestMu.Unlock()
		if draining {
			return
		}
		select {
		case <-deadline.C:
			t.Fatal("host did not close request admission")
		case <-ticker.C:
		}
	}
}
