package conf

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"sync"

	"github.com/BurntSushi/toml"
	"github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"
)

const (
	EnvServerConfigPath = "SERVER_CONFIG_PATH"
	DefaultConfigPath   = "conf/server/config.toml"
	defaultDeployMode   = DeployModeCloud
)

// Load reads a complete file with an independent parser. No process globals are changed.
func Load(path string) (*Config, error) {
	if path == "" {
		path = os.Getenv(EnvServerConfigPath)
		if path == "" {
			path = DefaultConfigPath
		}
	}
	v := viper.New()
	v.SetConfigFile(path)
	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("read configuration: %w", err)
	}
	return resolveConfig(v)
}

// FromSettings creates a validated configuration from caller-owned settings.
func FromSettings(settings map[string]any) (*Config, error) {
	v := viper.New()
	if err := v.MergeConfigMap(cloneMap(settings)); err != nil {
		return nil, err
	}
	return resolveConfig(v)
}

// resolveConfig freezes placeholder values before a configuration is decoded or published.
func resolveConfig(v *viper.Viper) (*Config, error) {
	settings, err := resolveSettings(v.AllSettings())
	if err != nil {
		return nil, fmt.Errorf("resolve configuration: %w", err)
	}
	resolved := viper.New()
	if err := resolved.MergeConfigMap(settings); err != nil {
		return nil, err
	}
	return decodeConfig(resolved)
}

func decodeConfig(v *viper.Viper) (*Config, error) {
	var config Config
	if err := v.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("decode configuration: %w", err)
	}
	if err := config.check(); err != nil {
		return nil, err
	}
	config.settings = cloneMap(v.AllSettings())
	// Store effective defaults in the snapshot so later reloads compare like values.
	data, err := toml.Marshal(config)
	if err != nil {
		return nil, err
	}
	effective := viper.New()
	effective.SetConfigType("toml")
	if err := effective.ReadConfig(bytes.NewReader(data)); err != nil {
		return nil, err
	}
	for _, key := range []string{"appname", "appcode", "runmode", "feature", "deploymode", "services", "httpserver", "fakeauth"} {
		delete(config.settings, key)
		if value, ok := effective.AllSettings()[key]; ok {
			config.settings[key] = cloneValue(value)
		}
	}
	return &config, nil
}

// UnmarshalKey decodes the effective values of a section in this snapshot.
func (c *Config) UnmarshalKey(key string, target any) error {
	if c == nil {
		return errors.New("configuration is nil")
	}
	v := viper.New()
	if err := v.MergeConfigMap(cloneMap(c.settings)); err != nil {
		return err
	}
	if !v.IsSet(key) {
		return fmt.Errorf("configuration section %s is missing", key)
	}
	return v.UnmarshalKey(key, target)
}

// Unmarshal decodes all snapshot settings without changing another configuration.
func (c *Config) Unmarshal(target any) error {
	if c == nil {
		return errors.New("configuration is nil")
	}
	v := viper.New()
	if err := v.MergeConfigMap(cloneMap(c.settings)); err != nil {
		return err
	}
	return v.Unmarshal(target)
}

// Clone transfers an independent snapshot to the caller.
func (c *Config) Clone() *Config {
	if c == nil {
		return nil
	}
	copy := *c
	copy.Services = append([]string(nil), c.Services...)
	copy.HTTPServer.Trace.PrefixFilters = append([]string(nil), c.HTTPServer.Trace.PrefixFilters...)
	copy.HTTPServer.Trace.SuffixFilters = append([]string(nil), c.HTTPServer.Trace.SuffixFilters...)
	copy.HTTPServer.ConsoleProxyNames = append([]string(nil), c.HTTPServer.ConsoleProxyNames...)
	copy.HTTPServer.AllowAPIPathNoHandlerAPIError = append(
		[]string(nil),
		c.HTTPServer.AllowAPIPathNoHandlerAPIError...)
	copy.HTTPServer.Assets = append([]HTTPServerAssetConfig(nil), c.HTTPServer.Assets...)
	copy.Feature = cloneValue(c.Feature)
	copy.settings = cloneMap(c.settings)
	return &copy
}

func cloneMap(input map[string]any) map[string]any {
	output := make(map[string]any, len(input))
	for key, value := range input {
		output[key] = cloneValue(value)
	}
	return output
}

func cloneValue(value any) any {
	switch v := value.(type) {
	case map[string]any:
		return cloneMap(v)
	case []any:
		out := make([]any, len(v))
		for i, value := range v {
			out[i] = cloneValue(value)
		}
		return out
	case []string:
		return append([]string(nil), v...)
	default:
		return value
	}
}

// Prepare validates a replacement and returns a non-failing commit action.
// It must not mutate live resources before returning successfully.
type Prepare func(previous, next *Config, changedKeys []string) (commit func(), err error)

// Manager owns configuration snapshots, reload subscriptions, and a file watcher.
type Manager struct {
	path     string
	mu       sync.RWMutex
	reloadMu sync.Mutex
	current  *Config
	handlers map[uint64]Prepare
	nextID   uint64
	cancel   context.CancelFunc
	done     chan struct{}
}

func NewManager(path string) (*Manager, error) {
	if path == "" {
		path = os.Getenv(EnvServerConfigPath)
		if path == "" {
			path = DefaultConfigPath
		}
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	config, err := Load(absolute)
	if err != nil {
		return nil, err
	}
	return &Manager{path: absolute, current: config, handlers: make(map[uint64]Prepare)}, nil
}

// NewManagerFromConfig constructs an instance from explicit, already effective values.
func NewManagerFromConfig(config *Config) (*Manager, error) {
	if config == nil {
		return nil, errors.New("configuration is nil")
	}
	data, err := toml.Marshal(config)
	if err != nil {
		return nil, err
	}
	v := viper.New()
	v.SetConfigType("toml")
	if err = v.ReadConfig(bytes.NewReader(data)); err != nil {
		return nil, err
	}
	normalized, err := decodeConfig(v)
	if err != nil {
		return nil, err
	}
	// A Config loaded from a file also carries application-specific sections.
	if config.settings != nil {
		values := cloneMap(config.settings)
		for _, key := range []string{"appname", "appcode", "runmode", "feature", "deploymode", "services", "httpserver", "fakeauth"} {
			delete(values, key)
			if value, exists := normalized.settings[key]; exists {
				values[key] = cloneValue(value)
			}
		}
		normalized.settings = values
	}
	return &Manager{current: normalized, handlers: make(map[uint64]Prepare)}, nil
}

func (m *Manager) Snapshot() *Config {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.current.Clone()
}

// Subscribe installs a staged replacement hook. Call the returned function to unsubscribe.
func (m *Manager) Subscribe(handler Prepare) func() {
	m.reloadMu.Lock()
	defer m.reloadMu.Unlock()
	return m.subscribe(handler)
}

// Bind initializes a subscriber from the current snapshot and then subscribes it
// under the same reload lock. A replacement cannot be missed during construction.
func (m *Manager) Bind(handler Prepare) (func(), error) {
	m.reloadMu.Lock()
	defer m.reloadMu.Unlock()
	current := m.Snapshot()
	commit, err := handler(current.Clone(), current, nil)
	if err != nil {
		return nil, err
	}
	if commit != nil {
		commit()
	}
	return m.subscribe(handler), nil
}

func (m *Manager) subscribe(handler Prepare) func() {
	m.nextID++
	id := m.nextID
	m.handlers[id] = handler
	return func() { m.reloadMu.Lock(); defer m.reloadMu.Unlock(); delete(m.handlers, id) }
}

func changedKeys(previous, next *Config) []string {
	keys := make(map[string]struct{})
	for k := range previous.settings {
		keys[k] = struct{}{}
	}
	for k := range next.settings {
		keys[k] = struct{}{}
	}
	var changed []string
	for key := range keys {
		if !reflect.DeepEqual(previous.settings[key], next.settings[key]) {
			changed = append(changed, key)
		}
	}
	sort.Strings(changed)
	return changed
}

// Update validates all subscribers before publishing a replacement.
func (m *Manager) Update(next *Config) error {
	if next == nil {
		return errors.New("configuration is nil")
	}
	normalized, err := NewManagerFromConfig(next)
	if err != nil {
		return err
	}
	m.reloadMu.Lock()
	defer m.reloadMu.Unlock()
	previous := m.Snapshot()
	candidate := normalized.Snapshot()
	keys := changedKeys(previous, candidate)
	if len(keys) == 0 {
		return nil
	}
	ids := make([]uint64, 0, len(m.handlers))
	for id := range m.handlers {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	var commits []func()
	for _, id := range ids {
		commit, err := m.handlers[id](previous.Clone(), candidate.Clone(), append([]string(nil), keys...))
		if err != nil {
			return err
		}
		if commit != nil {
			commits = append(commits, commit)
		}
	}
	m.mu.Lock()
	m.current = candidate
	m.mu.Unlock()
	for _, commit := range commits {
		commit()
	}
	return nil
}

func (m *Manager) Reload() error {
	if m.path == "" {
		return errors.New("configuration has no file path")
	}
	next, err := Load(m.path)
	if err != nil {
		return err
	}
	return m.Update(next)
}

// Watch starts one owned watcher. Replacement errors preserve the previous snapshot.
func (m *Manager) Watch(ctx context.Context, onError func(error)) error {
	if m.path == "" {
		return errors.New("configuration has no file path")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.cancel != nil {
		return errors.New("configuration watcher is already running")
	}
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	if err = watcher.Add(filepath.Dir(m.path)); err != nil {
		watcher.Close()
		return err
	}
	watchCtx, cancel := context.WithCancel(ctx)
	m.cancel = cancel
	m.done = make(chan struct{})
	go func() {
		defer close(m.done)
		defer watcher.Close()
		for {
			select {
			case <-watchCtx.Done():
				return
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if filepath.Clean(event.Name) != m.path ||
					event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Rename) == 0 {
					continue
				}
				if err := m.Reload(); err != nil && onError != nil {
					onError(err)
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				if onError != nil {
					onError(err)
				}
			}
		}
	}()
	return nil
}

// Close cancels and waits for the watcher; it is safe to call more than once.
func (m *Manager) Close(ctx context.Context) error {
	m.mu.RLock()
	cancel, done := m.cancel, m.done
	m.mu.RUnlock()
	if cancel == nil {
		return nil
	}
	cancel()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
