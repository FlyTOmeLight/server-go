package conf

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
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

func TestLoadAuthenticationDefaults(t *testing.T) {
	for _, test := range []struct {
		mode, configured string
		want             AuthType
	}{
		{mode: RunModeDebug, want: AuthTypeFake},
		{mode: RunModeTest, want: AuthTypeFake},
		{mode: RunModeRelease, want: ""},
		{mode: RunModeRelease, configured: "[HTTPServer]\nAuthType='Fake'\n", want: AuthTypeFake},
		{mode: RunModeRelease, configured: "[HTTPServer]\nAuthType='AuthCenter'\n", want: AuthTypeAuthCenter},
	} {
		path := filepath.Join(t.TempDir(), "config.toml")
		content := fmt.Sprintf("AppName='test'\nAppCode=100\nRunMode='%s'\n%s", test.mode, test.configured)
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		config, err := Load(path)
		if err != nil {
			t.Fatal(err)
		}
		if got := config.HTTPServer.AuthType; got != test.want {
			t.Errorf(
				"mode=%s config=%q: AuthType = %q, want %q",
				test.mode,
				test.configured,
				got,
				test.want,
			)
		}
	}
}

func TestHTTPServerConfigValidation(t *testing.T) {
	for _, test := range []struct {
		name, settings, errorField string
	}{
		{name: "negative read timeout", settings: "ReadTimeout=-1", errorField: "ReadTimeout"},
		{name: "negative write timeout", settings: "WriteTimeout=-1", errorField: "WriteTimeout"},
		{name: "negative idle timeout", settings: "IdleTimeout=-1", errorField: "IdleTimeout"},
		{name: "negative header timeout", settings: "ReadHeaderTimeout=-1", errorField: "ReadHeaderTimeout"},
		{name: "negative prestop delay", settings: "PreStopDelay=-1", errorField: "PreStopDelay"},
		{name: "negative header size", settings: "MaxHeaderBytes=-1", errorField: "MaxHeaderBytes"},
		{name: "missing certificate", settings: "[HTTPServer.TLS]\nEnable=true\nKeyFile='key.pem'", errorField: "TLS.CertFile"},
		{name: "missing key", settings: "[HTTPServer.TLS]\nEnable=true\nCertFile='cert.pem'", errorField: "TLS.KeyFile"},
		{name: "TLS disabled", settings: "[HTTPServer.TLS]\nEnable=false"},
		{name: "same address", settings: "Listen='127.0.0.1:9090'\nAdminListen='127.0.0.1:9090'", errorField: "AdminListen"},
		{name: "same ephemeral port", settings: "Listen='127.0.0.1:0'\nAdminListen='127.0.0.1:0'"},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := Load(configFile(t, "[HTTPServer]\n"+test.settings+"\n"))
			if test.errorField == "" {
				if err != nil {
					t.Fatal(err)
				}
			} else if err == nil || !strings.Contains(err.Error(), "HTTPServer."+test.errorField) {
				t.Fatalf("expected validation error for %s, got %v", test.errorField, err)
			}
		})
	}
}

func TestLoadSwaggerDocPrefix(t *testing.T) {
	for _, test := range []struct{ config, want string }{
		{config: "[HTTPServer]\nSwaggerEnable=true\n"},
		{config: "[HTTPServer]\nSwaggerEnable=true\nSwaggerDocPrefix='configured-doc'\n", want: "configured-doc"},
	} {
		config, err := Load(configFile(t, test.config))
		if err != nil {
			t.Fatal(err)
		}
		if got := config.HTTPServer.SwaggerDocPrefix; got != test.want {
			t.Errorf("SwaggerDocPrefix = %q, want %q", got, test.want)
		}
	}
}

func TestHTTPServerConfigSnapshotOwnsTrustedProxies(t *testing.T) {
	manager, err := NewManager(configFile(t, `[HTTPServer]
Listen="127.0.0.1:8080"
AdminListen="127.0.0.1:9090"
ReadHeaderTimeout=2500
PreStopDelay=3000
TrustedProxies=["192.0.2.0/24", "2001:db8::1"]
[HTTPServer.TLS]
Enable=true
CertFile="cert.pem"
KeyFile="key.pem"
ClientCAFile="client-ca.pem"
`))
	if err != nil {
		t.Fatal(err)
	}
	snapshot := manager.Snapshot()
	want := HTTPServerConfig{
		Listen: "127.0.0.1:8080", AdminListen: "127.0.0.1:9090",
		ReadHeaderTimeout: 2500, PreStopDelay: 3000,
		TrustedProxies: []string{"192.0.2.0/24", "2001:db8::1"},
		TLS: HTTPServerTLSConfig{
			Enable: true, CertFile: "cert.pem", KeyFile: "key.pem", ClientCAFile: "client-ca.pem",
		},
	}
	if !reflect.DeepEqual(snapshot.HTTPServer, want) {
		t.Fatalf("HTTPServer = %#v, want %#v", snapshot.HTTPServer, want)
	}
	snapshot.HTTPServer.TrustedProxies[0] = "198.51.100.0/24"
	var decoded HTTPServerConfig
	if err := manager.Snapshot().UnmarshalKey("HTTPServer", &decoded); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(manager.Snapshot().HTTPServer, want) || !reflect.DeepEqual(decoded, want) {
		t.Fatal("editing the snapshot changed the manager's proxy list")
	}
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

func TestNilSubscribers(t *testing.T) {
	manager, err := NewManager(configFile(t, ""))
	if err != nil {
		t.Fatal(err)
	}
	if unsubscribe, err := manager.Bind(nil); err == nil || unsubscribe != nil {
		t.Fatal("Bind accepted a nil subscriber")
	}
	unsubscribe := manager.Subscribe(nil)
	next := manager.Snapshot()
	next.AppName = "updated"
	if err := manager.Update(next); err != nil {
		t.Fatal(err)
	}
	unsubscribe()
	unsubscribe()
}

func TestCloneCopiesTypedCollections(t *testing.T) {
	type names map[string]string
	feature := map[string]any{
		"labels": names{"env": "test"},
		"ports":  []int{80},
		"items":  []map[string]any{{"paths": []string{"/ping"}, "empty": nil}},
		"nested": map[int][1][]int{1: {{2}}},
		"nil":    []int(nil),
	}
	snapshot := (&Config{Feature: feature, settings: feature}).Clone()
	cloned := snapshot.Feature.(map[string]any)
	if !reflect.DeepEqual(cloned, feature) || !reflect.DeepEqual(snapshot.settings, feature) {
		t.Fatalf("clone changed collection values or types: %#v", cloned)
	}
	cloned["labels"].(names)["env"] = "production"
	cloned["ports"].([]int)[0] = 443
	cloned["items"].([]map[string]any)[0]["paths"].([]string)[0] = "/other"
	snapshot.settings["nested"].(map[int][1][]int)[1][0][0] = 3
	if feature["labels"].(names)["env"] != "test" || feature["ports"].([]int)[0] != 80 ||
		feature["items"].([]map[string]any)[0]["paths"].([]string)[0] != "/ping" ||
		feature["nested"].(map[int][1][]int)[1][0][0] != 2 {
		t.Fatalf("clone shares storage with the original: %#v", feature)
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

func TestWatchConfigMapSymlinkReload(t *testing.T) {
	dir := t.TempDir()
	writeVersion := func(version, appName string) {
		t.Helper()
		versionDir := filepath.Join(dir, version)
		if err := os.Mkdir(versionDir, 0o700); err != nil {
			t.Fatal(err)
		}
		contents := fmt.Sprintf("AppName=%q\nAppCode=100\nRunMode='release'\n", appName)
		if err := os.WriteFile(filepath.Join(versionDir, "config.toml"), []byte(contents), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	link := func(target, name string) {
		t.Helper()
		if err := os.Symlink(target, filepath.Join(dir, name)); err != nil {
			t.Fatal(err)
		}
	}
	writeVersion("..old", "old")
	link("..old", "..data")
	link(filepath.Join("..data", "config.toml"), "config.toml")
	manager, err := NewManager(filepath.Join(dir, "config.toml"))
	if err != nil {
		t.Fatal(err)
	}
	updated := make(chan string, 1)
	manager.Subscribe(func(_, next *Config, _ []string) (func(), error) {
		return func() { updated <- next.AppName }, nil
	})
	if err := manager.Watch(context.Background(), func(err error) { t.Error(err) }); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = manager.Close(context.Background()) })
	writeVersion("..new", "new")
	link("..new", "..data_tmp")
	if err := os.Rename(filepath.Join(dir, "..data_tmp"), filepath.Join(dir, "..data")); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(filepath.Join(dir, "..old")); err != nil {
		t.Fatal(err)
	}
	select {
	case got := <-updated:
		if got != "new" {
			t.Fatalf("reloaded AppName = %q, want new", got)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("watcher did not reload the new ConfigMap version")
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
