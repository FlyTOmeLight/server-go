package cli

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/FlyTOmeLight/server-go/sdk"
)

type testCommandBuilder struct{ newCommand func() *cobra.Command }

func (b testCommandBuilder) NewCommand() *cobra.Command { return b.newCommand() }

func configuredLine(t *testing.T, endpoint string) *CommandLine {
	t.Helper()
	line := NewCommandLineWithName("test")
	file := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(
		file,
		[]byte("core:\n  endpoint: "+endpoint+"\nlog:\n  handler: NONE\n"),
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	line.SetConfigFile(file)
	return line
}

func TestCommandLinesIsolateConfigurationAndApplyHandlers(t *testing.T) {
	var lines []*CommandLine
	var counts [2]int
	for i := range counts {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != fmt.Sprintf("/handler%d", i) {
				t.Errorf("wrong handler for instance %d: %s", i, r.URL.Path)
			}
			counts[i]++
			w.WriteHeader(http.StatusNoContent)
		}))
		t.Cleanup(server.Close)
		line := configuredLine(t, server.URL)
		line.Append(testCommandBuilder{newCommand: func() *cobra.Command {
			cmd := &cobra.Command{Use: "domain"}
			RegisterCommandApplyHandler(
				cmd,
				"Shared",
				&struct{}{},
				func(ctx context.Context, input any) error {
					client, err := Client(ctx)
					if err != nil {
						return err
					}
					response, err := client.R().SetContext(ctx).Get(fmt.Sprintf("/handler%d", i))
					return sdk.Complete(response, err)
				},
			)
			return cmd
		}})
		filename := filepath.Join(t.TempDir(), "apply.yaml")
		if err := os.WriteFile(filename, []byte("kind: shared\nmetadata: {}\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		line.Root.SetArgs([]string{"apply", "-f", filename})
		lines = append(lines, line)
	}
	var wait sync.WaitGroup
	for _, line := range lines {
		wait.Go(func() {
			if err := line.ExecuteContext(context.Background()); err != nil {
				t.Error(err)
			}
		})
	}
	wait.Wait()
	for i, line := range lines {
		if counts[i] != 1 {
			t.Fatalf("instance %d handled %d requests", i, counts[i])
		}
		if _, err := Client(line.Root.Context()); err == nil {
			t.Fatal("client remains available after execution")
		}
	}
}

func TestCommandExecutionCancellationAndRepeatedExecution(t *testing.T) {
	started := make(chan struct{})
	canceled := make(chan struct{})
	var once sync.Once
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/done" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		once.Do(func() { close(started) })
		<-r.Context().Done()
		close(canceled)
	}))
	defer server.Close()
	line := configuredLine(t, server.URL)
	path := "/wait"
	line.Append(testCommandBuilder{newCommand: func() *cobra.Command {
		return &cobra.Command{Use: "request", RunE: func(cmd *cobra.Command, args []string) error {
			client, err := Client(cmd.Context())
			if err != nil {
				return err
			}
			response, err := client.R().SetContext(cmd.Context()).Get(path)
			return sdk.Complete(response, err)
		}}
	}})
	line.Root.SetArgs([]string{"request"})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	result := make(chan error, 1)
	go func() { result <- line.ExecuteContext(ctx) }()
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("request did not start")
	}
	cancel()
	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("cancellation lost: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("execution did not stop")
	}
	select {
	case <-canceled:
	case <-time.After(3 * time.Second):
		t.Fatal("server request did not stop")
	}
	if _, err := Client(line.Root.Context()); err == nil {
		t.Fatal("failed execution retained its client")
	}
	path = "/done"
	if err := line.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("second execution inherited canceled context: %v", err)
	}
}

func TestLegacyApplyRegistrationsAreSnapshotted(t *testing.T) {
	previous := handlers
	handlers = nil
	t.Cleanup(func() { handlers = previous })
	calls := 0
	RegisterApplyHandler("Legacy", &struct{}{}, func(any) error { calls++; return nil })
	line := configuredLine(t, "http://localhost:1")
	RegisterApplyHandler(
		"Legacy",
		&struct{}{},
		func(any) error { t.Fatal("later registration replaced snapshot"); return nil },
	)
	filename := filepath.Join(t.TempDir(), "legacy.yaml")
	if err := os.WriteFile(filename, []byte("kind: Legacy\nmetadata: {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	line.Root.SetArgs([]string{"apply", "-f", filename})
	if err := line.ExecuteContext(context.Background()); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("legacy handler called %d times", calls)
	}
}

func TestConfigCommandsUseTheirOwnFiles(t *testing.T) {
	first, second := NewCommandLineWithName("first"), NewCommandLineWithName("second")
	files := []string{
		filepath.Join(t.TempDir(), "first", "config.yaml"),
		filepath.Join(t.TempDir(), "second", "config.yaml"),
	}
	for i, line := range []*CommandLine{first, second} {
		line.SetConfigFile(files[i])
		line.Root.SetArgs([]string{"config", "set", fmt.Sprintf("core.endpoint=http://endpoint%d", i)})
		if err := line.ExecuteContext(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	for i, file := range files {
		config := viper.New()
		config.SetConfigFile(file)
		if err := config.ReadInConfig(); err != nil {
			t.Fatal(err)
		}
		if got := config.GetString("core.endpoint"); got != fmt.Sprintf("http://endpoint%d", i) {
			t.Fatalf("config changed another file: %s", got)
		}
	}
}

func TestStandaloneVersionCommandUsesManualConfiguration(t *testing.T) {
	previous := Config
	t.Cleanup(func() {
		if Config.Client != nil {
			_ = Config.Client.Close()
		}
		Config = previous
		viper.Reset()
	})
	viper.Reset()
	requested := make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/version" {
			t.Errorf("version path = %s", r.URL.Path)
		}
		requested <- struct{}{}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"AppVersion":"manual"}`))
	}))
	defer server.Close()
	file := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(
		file,
		[]byte("core:\n  endpoint: "+server.URL+"\nlog:\n  handler: NONE\n"),
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	viper.SetConfigFile(file)
	root := &cobra.Command{Use: "manual"}
	root.AddCommand(NewVersionCommand())
	root.SetArgs([]string{"version", "--output=json"})
	if err := root.ExecuteContext(context.Background()); err != nil {
		t.Fatal(err)
	}
	select {
	case <-requested:
	default:
		t.Fatal("manual version command did not request server version")
	}
}
