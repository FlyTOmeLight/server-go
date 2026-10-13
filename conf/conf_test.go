package conf

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"time"
)

func configFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(
		path,
		[]byte("AppName='test'\nAppCode=100\nRunMode='release'\n"+content),
		0o600,
	); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadAndIsolation(t *testing.T) {
	path := configFile(t, "[Feature]\nOld=true\n[DB]\nDriver='SQLite'\nDB='one.db'\n")
	manager, err := NewManager(path)
	if err != nil {
		t.Fatal(err)
	}
	old := manager.Snapshot()
	old.AppName = "changed"
	old.Feature.(map[string]any)["old"] = false
	if manager.Snapshot().AppName != "test" || manager.Snapshot().Feature.(map[string]any)["old"] != true {
		t.Fatal("snapshot mutation leaked")
	}
	var section struct{ Driver, DB string }
	if err := manager.Snapshot().UnmarshalKey("DB", &section); err != nil {
		t.Fatal(err)
	}
	if section.DB != "one.db" {
		t.Fatal(section)
	}
	var keys []string
	manager.Subscribe(
		func(previous, next *Config, changed []string) (func(), error) { keys = changed; return nil, nil },
	)
	os.WriteFile(
		path,
		[]byte("AppName='test'\nAppCode=100\nRunMode='release'\n[NewSection]\nEnabled=true\n"),
		0o600,
	)
	if err := manager.Reload(); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(keys, []string{"db", "feature", "newsection"}) {
		t.Fatalf("keys = %v", keys)
	}
	if err := manager.Snapshot().UnmarshalKey("DB", &section); err == nil {
		t.Fatal("deleted section remains")
	}
	if old.settings["db"] == nil {
		t.Fatal("old snapshot changed")
	}
}

func TestInvalidReplacementPreservesSnapshot(t *testing.T) {
	path := configFile(t, "")
	manager, err := NewManager(path)
	if err != nil {
		t.Fatal(err)
	}
	os.WriteFile(path, []byte("AppName=''"), 0o600)
	for i := 0; i < 2; i++ {
		if err := manager.Reload(); err == nil {
			t.Fatal("invalid reload accepted")
		}
	}
	if manager.Snapshot().AppName != "test" {
		t.Fatal("valid configuration lost")
	}
	for i := 0; i < 2; i++ {
		if _, err := NewManager(filepath.Join(t.TempDir(), "missing")); err == nil {
			t.Fatal("missing file accepted")
		}
	}
}

func TestManagerFromLoadedConfigPreservesSections(t *testing.T) {
	config, err := Load(configFile(t, "[DB]\nDriver='SQLite'\nDB='one.db'\n"))
	if err != nil {
		t.Fatal(err)
	}
	manager, err := NewManagerFromConfig(config)
	if err != nil {
		t.Fatal(err)
	}
	var section struct{ Driver, DB string }
	if err := manager.Snapshot().UnmarshalKey("DB", &section); err != nil {
		t.Fatal(err)
	}
	if section.DB != "one.db" {
		t.Fatal(section)
	}
}

func TestConcurrentSnapshots(t *testing.T) {
	manager, err := NewManagerFromConfig(&Config{AppName: "test", AppCode: 100, RunMode: "release"})
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				snapshot := manager.Snapshot()
				snapshot.AppName = "other"
				if err := manager.Update(snapshot); err != nil {
					t.Error(err)
				}
			}
		}()
	}
	wg.Wait()
}

func TestWatchAndClose(t *testing.T) {
	path := configFile(t, "")
	manager, err := NewManager(path)
	if err != nil {
		t.Fatal(err)
	}
	changed := make(chan struct{}, 1)
	manager.Subscribe(func(old, next *Config, _ []string) (func(), error) {
		return func() {
			select {
			case changed <- struct{}{}:
			default:
			}
		}, nil
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := manager.Watch(ctx, func(err error) { t.Log(err) }); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(path, []byte("AppName='updated'\nAppCode=100\nRunMode='release'\n"), 0o600)
	select {
	case <-changed:
	case <-time.After(3 * time.Second):
		t.Fatal("watcher did not reload")
	}
	if manager.Snapshot().AppName != "updated" {
		t.Fatal("replacement missing")
	}
	if err := manager.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := manager.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestPlaceholderSnapshotsAndReload(t *testing.T) {
	for _, source := range []string{"env", "file"} {
		t.Run(source, func(t *testing.T) {
			var placeholder string
			var setValue func(string)
			if source == "env" {
				placeholder = "${env: SERVER_GO_SNAPSHOT_TEST}"
				setValue = func(value string) { t.Setenv("SERVER_GO_SNAPSHOT_TEST", value) }
			} else {
				secretPath := filepath.Join(t.TempDir(), "secret")
				placeholder = "${file: " + secretPath + "}"
				setValue = func(value string) {
					if err := os.WriteFile(secretPath, []byte(value+"\n"), 0o600); err != nil {
						t.Fatal(err)
					}
				}
			}
			setValue("first")
			manager, err := NewManager(configFile(t, fmt.Sprintf("[Business]\nPassword=%q\n", placeholder)))
			if err != nil {
				t.Fatal(err)
			}
			previous := manager.Snapshot()
			read := func(config *Config) string {
				t.Helper()
				var business struct{ Password string }
				if err := config.UnmarshalKey("Business", &business); err != nil {
					t.Fatal(err)
				}
				return business.Password
			}
			setValue("second")
			if got := read(previous); got != "first" {
				t.Fatalf("snapshot reread its source: %q", got)
			}
			if got := read(manager.Snapshot()); got != "first" {
				t.Fatalf("unpublished source change: %q", got)
			}
			rejected := errors.New("business change requires restart")
			unsubscribe := manager.Subscribe(func(old, next *Config, keys []string) (func(), error) {
				if read(old) != "first" || read(next) != "second" ||
					!reflect.DeepEqual(keys, []string{"business"}) {
					t.Fatalf(
						"unexpected reload: old=%q next=%q keys=%v",
						read(old),
						read(next),
						keys,
					)
				}
				return nil, rejected
			})
			if err := manager.Reload(); !errors.Is(err, rejected) {
				t.Fatalf("effective change was not validated: %v", err)
			}
			if got := read(manager.Snapshot()); got != "first" {
				t.Fatalf("rejected replacement published: %q", got)
			}
			unsubscribe()
			if err := manager.Reload(); err != nil {
				t.Fatal(err)
			}
			if read(manager.Snapshot()) != "second" || read(previous) != "first" {
				t.Fatal("reload did not preserve independent effective snapshots")
			}
		})
	}
}

func TestPlaceholdersResolveOnceBeforeDecoding(t *testing.T) {
	t.Setenv("SERVER_GO_LITERAL_TEST", "${env: SERVER_GO_INNER_TEST}")
	t.Setenv("SERVER_GO_INNER_TEST", "must not replace literal")
	t.Setenv("SERVER_GO_PORT_TEST", "1234")
	config, err := FromSettings(map[string]any{
		"AppName": "${env: SERVER_GO_LITERAL_TEST}", "AppCode": 100, "RunMode": "test",
		"Business": map[string]any{
			"Names": []any{"${env: SERVER_GO_LITERAL_TEST}"},
			"Port":  "${env: SERVER_GO_PORT_TEST}",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	manager, err := NewManagerFromConfig(config)
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.Update(manager.Snapshot()); err != nil {
		t.Fatal(err)
	}
	var decoded struct {
		AppName  string
		Business struct {
			Names []string
			Port  int
		}
	}
	if err := manager.Snapshot().Unmarshal(&decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.AppName != "${env: SERVER_GO_INNER_TEST}" ||
		!reflect.DeepEqual(decoded.Business.Names, []string{decoded.AppName}) || decoded.Business.Port != 1234 {
		t.Fatalf("effective values changed during decoding: %+v", decoded)
	}
}

func TestUnresolvedBusinessPlaceholderRejectsLoad(t *testing.T) {
	path := configFile(
		t,
		fmt.Sprintf("[Business]\nPassword=%q\n", "${file: "+filepath.Join(t.TempDir(), "missing")+"}"),
	)
	if _, err := Load(path); err == nil {
		t.Fatal("missing business placeholder source accepted")
	}
}

func TestTypedPlaceholderInputs(t *testing.T) {
	t.Setenv("SERVER_GO_TYPED_INPUT_TEST", "first")
	business := map[string]string{"Password": "${env: SERVER_GO_TYPED_INPUT_TEST}"}
	config, err := FromSettings(map[string]any{
		"AppName": "test", "AppCode": 100, "RunMode": "test",
		"Business": business,
		"Tokens":   []map[string]string{{"Password": "${env: SERVER_GO_TYPED_INPUT_TEST}"}},
		"Labels":   map[string][]string{"Names": {"${env: SERVER_GO_TYPED_INPUT_TEST}"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("SERVER_GO_TYPED_INPUT_TEST", "second")
	business["Password"] = "caller changed input"
	var decoded struct {
		Business struct{ Password string }
		Tokens   []struct{ Password string }
		Labels   struct{ Names []string }
	}
	if err := config.Unmarshal(&decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Business.Password != "first" || len(decoded.Tokens) != 1 ||
		decoded.Tokens[0].Password != "first" || !reflect.DeepEqual(decoded.Labels.Names, []string{"first"}) {
		t.Fatalf("typed input was not resolved into an independent snapshot: %+v", decoded)
	}
}
