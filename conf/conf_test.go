package conf

import (
	"context"
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
	if err := os.WriteFile(path, []byte("AppName='test'\nAppCode=100\nRunMode='release'\n"+content), 0600); err != nil {
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
	manager.Subscribe(func(previous, next *Config, changed []string) (func(), error) { keys = changed; return nil, nil })
	os.WriteFile(path, []byte("AppName='test'\nAppCode=100\nRunMode='release'\n[NewSection]\nEnabled=true\n"), 0600)
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
	os.WriteFile(path, []byte("AppName=''"), 0600)
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
	os.WriteFile(path, []byte("AppName='updated'\nAppCode=100\nRunMode='release'\n"), 0600)
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
