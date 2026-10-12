package conf

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSetDefault 测试 SetDefault 方法
func TestSetDefault(t *testing.T) {
	resetTestEnv := func() {
		viper.Reset()
		defaultConfig = nil
	}

	t.Run("nil config should return error", func(t *testing.T) {
		resetTestEnv()
		err := SetDefault(nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "config is nil")
	})

	t.Run("invalid config should return error", func(t *testing.T) {
		resetTestEnv()
		invalidConfig := &Config{}
		err := SetDefault(invalidConfig)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "CheckConfigError")
	})

	t.Run("valid config should be set correctly", func(t *testing.T) {
		resetTestEnv()
		validConfig := &Config{
			AppCode: 100,
			AppName: "testapp",
			RunMode: "debug",
		}

		err := SetDefault(validConfig)
		assert.NoError(t, err)

		assert.NotNil(t, defaultConfig)
		assert.Equal(t, "testapp", defaultConfig.AppName)
		assert.Equal(t, "debug", defaultConfig.RunMode)

		assert.Equal(t, "testapp", viper.Get("AppName"))
		assert.Equal(t, "debug", viper.Get("RunMode"))

		assert.Equal(t, LoadModeLoaded, defaultConfig.LoadMode)
	})

	t.Run("update existing config", func(t *testing.T) {
		resetTestEnv()
		initialConfig := &Config{
			AppCode: 100,
			AppName: "initialapp",
			RunMode: "release",
		}

		err := SetDefault(initialConfig)
		assert.NoError(t, err)

		// 然后更新配置
		updatedConfig := &Config{
			AppCode: 100,
			AppName: "updatedapp",
			RunMode: "debug",
		}

		err = SetDefault(updatedConfig)
		assert.NoError(t, err)

		assert.Equal(t, "updatedapp", defaultConfig.AppName)
		assert.Equal(t, "debug", defaultConfig.RunMode)

		assert.Equal(t, "updatedapp", viper.Get("AppName"))
		assert.Equal(t, "debug", viper.Get("RunMode"))
	})

	t.Run("integration with Initialize", func(t *testing.T) {
		// 重置测试环境
		viper.Reset()
		defaultConfig = nil
		once = sync.Once{} // 重置 sync.Once

		tmpDir, err := os.MkdirTemp("", "config_test")
		assert.NoError(t, err)
		defer os.RemoveAll(tmpDir)

		confDir := filepath.Join(tmpDir, "conf", "server")
		err = os.MkdirAll(confDir, 0755)
		assert.NoError(t, err)

		configPath := filepath.Join(confDir, "config.toml")
		err = os.WriteFile(configPath, []byte(`
AppName = "fileapp"
AppCode = 100
RunMode = "debug"
DeployMode = "Cloud"

[HTTPServer]
Listen = ":8080"
ReadTimeout = 5000
WriteTimeout = 10000
`), 0644)
		assert.NoError(t, err)

		oldWd, err := os.Getwd()
		assert.NoError(t, err)
		defer os.Chdir(oldWd)

		err = os.Chdir(tmpDir)
		assert.NoError(t, err)

		t.Run("Initialize first, then SetDefault", func(t *testing.T) {
			viper.Reset()
			defaultConfig = nil
			once = sync.Once{} // 重置 sync.Once

			err = Initialize()
			assert.NoError(t, err)
			assert.NotNil(t, defaultConfig)
			assert.Equal(t, "fileapp", defaultConfig.AppName)
			assert.Equal(t, "debug", defaultConfig.RunMode)
			assert.Equal(t, DeployModeCloud, defaultConfig.DeployMode)
			assert.Equal(t, ":8080", defaultConfig.HTTPServer.Listen)

			newConfig := &Config{
				AppCode:    100,
				AppName:    "newapp",
				RunMode:    "release",
				DeployMode: DeployModeCloud,
				HTTPServer: HTTPServerConfig{
					Listen:       ":9090",
					ReadTimeout:  3000,
					WriteTimeout: 6000,
				},
			}

			err = SetDefault(newConfig)
			assert.NoError(t, err)

			assert.Equal(t, "newapp", defaultConfig.AppName)
			assert.Equal(t, "release", defaultConfig.RunMode)
			assert.Equal(t, DeployModeCloud, defaultConfig.DeployMode)
			assert.Equal(t, ":9090", defaultConfig.HTTPServer.Listen)
			assert.Equal(t, 3000, defaultConfig.HTTPServer.ReadTimeout)

			assert.Equal(t, "newapp", viper.GetString("AppName"))
			assert.Equal(t, "release", viper.GetString("RunMode"))
			assert.Equal(t, string(DeployModeCloud), viper.GetString("DeployMode"))
			assert.Equal(t, ":9090", viper.GetString("HTTPServer.Listen"))
		})

		t.Run("SetDefault first, then Initialize", func(t *testing.T) {
			// 重置测试环境
			viper.Reset()
			defaultConfig = nil
			once = sync.Once{}

			// 创建新的配置对象
			newConfig := &Config{
				AppCode:    100,
				AppName:    "defaultapp",
				RunMode:    "test",
				DeployMode: DeployModeEdge,
				HTTPServer: HTTPServerConfig{
					Listen:       ":7070",
					ReadTimeout:  2000,
					WriteTimeout: 4000,
				},
			}

			err = SetDefault(newConfig)
			assert.NoError(t, err)
			assert.NotNil(t, defaultConfig)
			assert.Equal(t, "defaultapp", defaultConfig.AppName)
			assert.Equal(t, "test", defaultConfig.RunMode)
			assert.Equal(t, DeployModeEdge, defaultConfig.DeployMode)

			err = Initialize()
			assert.NoError(t, err)

			assert.Equal(t, "fileapp", defaultConfig.AppName)
			assert.Equal(t, "debug", defaultConfig.RunMode)
			assert.Equal(t, DeployModeCloud, defaultConfig.DeployMode)
			assert.Equal(t, ":8080", defaultConfig.HTTPServer.Listen)

			assert.Equal(t, "fileapp", viper.GetString("AppName"))
		})

		t.Run("config file change", func(t *testing.T) {
			viper.Reset()
			defaultConfig = nil
			once = sync.Once{} // 重置 sync.Once

			var changedConfig *Config
			var changedKeys []string
			callbackCalled := make(chan bool, 1)

			err = Initialize()
			assert.NoError(t, err)

			RegisterConfigContentChanged(func(newConfig *Config, keys []string) {
				changedConfig = newConfig
				changedKeys = keys
				callbackCalled <- true
			})

			err = os.WriteFile(configPath, []byte(`
AppName = "updatedapp"
AppCode = 100
RunMode = "release"
DeployMode = "Cloud"

[HTTPServer]
Listen = ":9090"
ReadTimeout = 3000
WriteTimeout = 6000
`), 0644)
			assert.NoError(t, err)

			event := fsnotify.Event{
				Name: defaultConfigFilePath,
				Op:   fsnotify.Write,
			}
			onConfigChangedHandler(event)

			select {
			case <-callbackCalled:
				assert.NotNil(t, changedConfig)
				assert.Contains(t, changedKeys, "appname")
				assert.Equal(t, "updatedapp", defaultConfig.AppName)
				assert.Equal(t, "release", defaultConfig.RunMode)
				assert.Equal(t, DeployModeCloud, defaultConfig.DeployMode)
			case <-time.After(2 * time.Second):
				t.Fatal("Callback was not called within timeout")
			}
		})
	})
}

func TestGetClient(t *testing.T) {
	viper.Set("test-client", map[string]any{
		"endpoint":  "test-endpoint",
		"accessKey": "test-accessKey",
		"secretKey": "test-secretKey",
	})

	tests := []struct {
		name  string
		empty bool
	}{
		{
			name:  "test-client",
			empty: false,
		},
		{
			name:  "not-exist",
			empty: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetClient(tt.name)
			if tt.empty {
				require.Nil(t, got)
			} else {
				require.NotNil(t, got)
			}
		})
	}
}
