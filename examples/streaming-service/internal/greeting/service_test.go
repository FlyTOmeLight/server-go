package greeting

import (
	"bufio"
	"context"
	"errors"
	"io"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/FlyTOmeLight/server-go/conf"
	"github.com/FlyTOmeLight/server-go/console"
	serrors "github.com/FlyTOmeLight/server-go/errors"
	clientapi "github.com/FlyTOmeLight/server-go/examples/streaming-service/sdk/greeting"
	"github.com/FlyTOmeLight/server-go/host"
	"github.com/FlyTOmeLight/server-go/httpintegration"
	"github.com/FlyTOmeLight/server-go/logger"
)

func TestStreamFlushDeadlineAndClose(t *testing.T) {
	for _, mode := range []string{"direct", "console", "auto-error", "cancel"} {
		t.Run(mode, func(t *testing.T) {
			next := make(chan struct{})
			stopped := make(chan struct{})
			generate := func(ctx context.Context, name string, emit func(string) error) error {
				defer close(stopped)
				if err := emit("hello " + name); err != nil {
					return err
				}
				select {
				case <-next:
				case <-ctx.Done():
					return ctx.Err()
				}
				if err := emit("second"); err != nil {
					return err
				}
				<-ctx.Done()
				return ctx.Err()
			}
			basePath := "/streaming-service/internal"
			if mode == "auto-error" {
				basePath = "/streaming-service/api"
			}
			manager, err := conf.NewManagerFromConfig(&conf.Config{
				AppName: "streaming-service",
				AppCode: 100,
				RunMode: conf.RunModeDebug,
				HTTPServer: conf.HTTPServerConfig{
					APIBasePath:         basePath,
					AltoHandlerAPIError: mode == "auto-error",
				},
			})
			if err != nil {
				t.Fatal(err)
			}
			defer manager.Close(context.Background())
			log, err := logger.NewLogger(&logger.Config{})
			if err != nil {
				t.Fatal(err)
			}
			defer log.Close()
			h, err := host.NewHTTPHost(manager, log, httpintegration.Install, New(generate))
			if err != nil {
				t.Fatal(err)
			}
			defer h.Close()
			backend := httptest.NewUnstartedServer(h.Handler())
			backend.Config.WriteTimeout = 30 * time.Millisecond
			backend.Start()
			defer backend.Close()
			endpoint := backend.URL + basePath
			if mode == "console" {
				engine := gin.New()
				engine.GET("/streaming-service/api/*any", console.Reverse(&conf.HTTPServerConfig{
					Listen: strings.TrimPrefix(backend.URL, "http://"), WriteTimeout: 1000,
				}, 100, engine))
				frontend := httptest.NewServer(engine)
				defer frontend.Close()
				endpoint = frontend.URL + "/streaming-service/api"
			}
			client, err := clientapi.NewClient(endpoint)
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			body, err := client.Open(ctx, "reader")
			if err != nil {
				t.Fatal(err)
			}
			defer body.Close()
			reader := bufio.NewScanner(body)
			if !reader.Scan() || reader.Text() != "event:message" {
				t.Fatalf("first event: %q, %v", reader.Text(), reader.Err())
			}
			if !reader.Scan() || !strings.Contains(reader.Text(), "hello reader") {
				t.Fatalf("first data: %q", reader.Text())
			}
			// Hold the producer past the server's original total write deadline.
			<-time.After(70 * time.Millisecond)
			close(next)
			for reader.Scan() && !strings.Contains(reader.Text(), "second") {
			}

			if !strings.Contains(reader.Text(), "second") {
				t.Fatalf("second event: %q, %v", reader.Text(), reader.Err())
			}
			if mode == "cancel" {
				cancel()
			} else if err := body.Close(); err != nil {
				t.Fatal(err)
			}
			select {
			case <-stopped:
			case <-time.After(time.Second):
				t.Fatal("closing SDK stream did not cancel producer")
			}
		})
	}
}

func TestStreamCompletionAndErrors(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(map[bool]string{false: "complete", true: "error"}[fail], func(t *testing.T) {
			router := gin.New()
			router.Use(serrors.ErrorHandler(&conf.Config{}))
			svc := New(func(ctx context.Context, name string, emit func(string) error) error {
				if err := emit("first"); err != nil {
					return err
				}
				if fail {
					return errors.New("private upstream detail")
				}
				return nil
			})
			if err := svc.RegisterRoutes(router.Group("")); err != nil {
				t.Fatal(err)
			}
			server := httptest.NewServer(router)
			defer server.Close()
			client, err := clientapi.NewClient(server.URL)
			if err != nil {
				t.Fatal(err)
			}
			body, err := client.Open(context.Background(), "reader")
			if err != nil {
				t.Fatal(err)
			}
			defer body.Close()
			data, err := io.ReadAll(body)
			if err != nil {
				t.Fatal(err)
			}
			text := string(data)
			if !strings.Contains(text, "event:message") {
				t.Fatal(text)
			}
			if fail {
				if !strings.Contains(text, "event:error") || strings.Contains(text, "event:done") ||
					strings.Contains(text, "private upstream detail") {
					t.Fatal(text)
				}
			} else if !strings.HasSuffix(text, "event:done\ndata:{}\n\n") {
				t.Fatal(text)
			}
			var detail *serrors.Error
			if _, err := client.Open(
				context.Background(),
				strings.Repeat("x", 65),
			); !errors.As(err, &detail) ||
				detail.Status != 400 {
				t.Fatalf("invalid name did not return an HTTP error: %v", err)
			}
		})
	}
}
