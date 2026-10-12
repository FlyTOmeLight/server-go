package cli

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"

	"github.com/FlyTOmeLight/server-go/sdk"
)

func TestSetConfigRejectsUnsupportedAKSK(t *testing.T) {
	for _, filename := range []string{"full_config.yaml", "default_config.yaml"} {
		t.Run(filename, func(t *testing.T) {
			viper.Reset()
			t.Cleanup(viper.Reset)
			viper.SetConfigFile(filepath.Join("testdata", filename))
			if err := SetConfig(); !errors.Is(err, sdk.ErrUnsupportedAuth) {
				t.Fatalf("expected unsupported AKSK error, got %v", err)
			}
		})
	}
}

func TestSetConfigAPIKeyAndInvalidModes(t *testing.T) {
	for _, test := range []struct {
		name      string
		core      string
		wantError bool
	}{
		{name: "API key", core: "endpoint: http://localhost:8001\n  api_key: token"},
		{name: "conflicting auth", core: "endpoint: http://localhost:8001\n  api_key: token\n  access_key: ak", wantError: true},
		{name: "relative endpoint", core: "endpoint: localhost:8001", wantError: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			viper.Reset()
			t.Cleanup(viper.Reset)
			filename := filepath.Join(t.TempDir(), "config.yaml")
			if err := os.WriteFile(
				filename,
				[]byte("core:\n  "+test.core+"\nlog:\n  handler: NONE\n"),
				0o600,
			); err != nil {
				t.Fatal(err)
			}
			viper.SetConfigFile(filename)
			err := SetConfig()
			if (err != nil) != test.wantError {
				t.Fatalf("got error %v, wantError %v", err, test.wantError)
			}
			if !test.wantError {
				client := Config.Client
				t.Cleanup(func() { _ = client.Close() })
				if client.Config.Credentials.APIKey != "token" {
					t.Fatal("client lost configured API key")
				}
			}
		})
	}
}
