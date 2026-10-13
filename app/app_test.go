package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	core "github.com/FlyTOmeLight/server-go/pkg"
)

type testModule struct {
	events *[]string
}

func (m *testModule) Initialize(context.Context) error {
	*m.events = append(*m.events, "initialize")
	return nil
}

func (m *testModule) Close(ctx context.Context) error {
	*m.events = append(*m.events, "close module")
	return ctx.Err()
}

type testService struct {
	events   *[]string
	name     string
	routeErr error
	startErr error
	stopErr  error
	onStart  func()
}

func (s *testService) RegisterRoutes(*core.RouterGroup) error {
	*s.events = append(*s.events, "routes "+s.name)
	return s.routeErr
}

func (s *testService) Start(context.Context) error {
	*s.events = append(*s.events, "start "+s.name)
	if s.onStart != nil {
		s.onStart()
	}
	return s.startErr
}

func (s *testService) Stop(ctx context.Context) error {
	*s.events = append(*s.events, "stop "+s.name)
	return errors.Join(s.stopErr, ctx.Err())
}

func testOptions(t *testing.T) Options {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(
		path,
		[]byte("AppName = 'test'\nAppCode = 100\nRunMode = 'release'\n[HTTPServer]\nListen = '127.0.0.1:0'\n"),
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	return Options{ConfigPath: path}
}

func TestRunOwnsServicesAndResourcesOnEveryExit(t *testing.T) {
	failure := errors.New("operation failed")
	stopFailure := errors.New("stop failed")
	for _, tc := range []struct {
		name     string
		buildErr error
		routeErr error
		startErr error
		stopErr  error
		want     []string
	}{
		{
			name: "assembly failure", buildErr: failure,
			want: []string{"initialize", "stop second", "stop first", "close module"},
		},
		{
			name: "route failure", routeErr: failure,
			want: []string{"initialize", "routes first", "stop second", "stop first", "close module"},
		},
		{
			name: "start failure", startErr: failure,
			want: []string{"initialize", "routes first", "routes second", "start first", "start second", "stop second", "stop first", "close module"},
		},
		{
			name: "canceled startup",
			want: []string{"initialize", "routes first", "routes second", "start first", "start second", "stop second", "stop first", "close module"},
		},
		{
			name: "shutdown error", stopErr: stopFailure,
			want: []string{"initialize", "routes first", "routes second", "start first", "start second", "stop second", "stop second"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			options := testOptions(t)
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			var events []string
			err := Run(ctx, options, func(initCtx context.Context, runtime *Runtime, _ *struct{}) error {
				if err := runtime.Initialize(initCtx, &testModule{events: &events}); err != nil {
					return err
				}
				if err := runtime.AddService(
					&testService{events: &events, name: "first", routeErr: tc.routeErr},
				); err != nil {
					return err
				}
				if err := runtime.AddService(
					&testService{
						events:   &events,
						name:     "second",
						startErr: tc.startErr,
						stopErr:  tc.stopErr,
						onStart:  cancel,
					},
				); err != nil {
					return err
				}
				return tc.buildErr
			})
			wantErr := errors.Join(tc.buildErr, tc.routeErr, tc.startErr, tc.stopErr)
			if tc.buildErr == nil && tc.routeErr == nil {
				wantErr = errors.Join(wantErr, context.Canceled)
			}
			if wantErr == nil && err != nil {
				t.Fatal(err)
			}
			for _, cause := range []error{tc.buildErr, tc.routeErr, tc.startErr, tc.stopErr} {
				if cause != nil && !errors.Is(err, cause) {
					t.Fatalf("lost error %v: %v", cause, err)
				}
			}
			if tc.buildErr == nil && tc.routeErr == nil && !errors.Is(err, context.Canceled) {
				t.Fatalf("startup cancellation error = %v", err)
			}
			if !reflect.DeepEqual(events, tc.want) {
				t.Fatalf("events = %v, want %v", events, tc.want)
			}
		})
	}
}

func TestInitializationDeadlineStillAllowsCleanup(t *testing.T) {
	options := testOptions(t)
	options.InitTimeout = 10 * time.Millisecond
	var events []string
	err := Run(context.Background(), options, func(ctx context.Context, runtime *Runtime, _ *struct{}) error {
		if err := runtime.Initialize(ctx, &testModule{events: &events}); err != nil {
			return err
		}
		if err := runtime.AddService(&testService{events: &events, name: "first"}); err != nil {
			return err
		}
		<-ctx.Done()
		return nil
	})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("deadline error = %v", err)
	}
	if want := []string{"initialize", "stop first", "close module"}; !reflect.DeepEqual(events, want) {
		t.Fatalf("events = %v, want %v", events, want)
	}
}

func TestRunDecodesTypedConfigBeforeAssembly(t *testing.T) {
	t.Setenv("APP_TEST_STORAGE_NAME", "storage")
	type config struct {
		Storage struct {
			Name string
			Port int
		}
	}
	for _, tc := range []struct {
		name    string
		port    string
		wantErr bool
	}{
		{name: "typed values and placeholders", port: "1234"},
		{name: "invalid field type", port: "'invalid'", wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			options := testOptions(t)
			file, err := os.OpenFile(options.ConfigPath, os.O_APPEND|os.O_WRONLY, 0o600)
			if err != nil {
				t.Fatal(err)
			}
			_, writeErr := file.WriteString(
				"\n[Storage]\nName = '${env:APP_TEST_STORAGE_NAME}'\nPort = " + tc.port + "\n",
			)
			if err := errors.Join(writeErr, file.Close()); err != nil {
				t.Fatal(err)
			}
			built := false
			assembled := errors.New("assembly complete")
			err = Run(
				context.Background(),
				options,
				func(_ context.Context, _ *Runtime, cfg *config) error {
					built = true
					if cfg.Storage.Name != "storage" || cfg.Storage.Port != 1234 {
						t.Fatalf("config = %#v", cfg)
					}
					return assembled
				},
			)
			if tc.wantErr {
				if err == nil || built {
					t.Fatalf("invalid configuration reached assembly: built=%v, err=%v", built, err)
				}
			} else if !errors.Is(err, assembled) || !built {
				t.Fatalf("assembly was not called: built=%v, err=%v", built, err)
			}
		})
	}
}
