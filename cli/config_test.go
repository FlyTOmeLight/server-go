package cli

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
)

func TestInitConfigFull(t *testing.T) {
	t.Run("initconfig_full", func(t *testing.T) {
		f := filepath.Join("testdata/full_config.yaml")
		f, err := filepath.Abs(f)
		if err != nil {
			t.Fatalf("GetFullPath err, %v", err)
		}

		fmt.Println(f)
		viper.SetConfigFile(f)
		err = viper.ReadInConfig()
		if err != nil {
			t.Fatalf("ReadInConfig err, %v", err)
		}

		err = SetConfig()
		if err != nil {
			t.Fatalf("initConfig err, %v", err)
		}

		cred := Config.Client.Config.Credentials
		if cred.AccessKeyId != "test-ak" {
			t.Fatalf("Credentials.AccessKeyId=%v, wanted: test-ak", cred.AccessKeyId)
		}

		if cred.SecretAccessKey != "test-sk" {
			t.Fatalf("Credentials.SecretAccessKey=%v, wanted: test-sk", cred.SecretAccessKey)
		}
	})

	t.Run("initconfig_", func(t *testing.T) {
		f := filepath.Join("testdata/default_config.yaml")
		f, err := filepath.Abs(f)
		if err != nil {
			t.Fatalf("GetFullPath err, %v", err)
		}

		fmt.Println(f)
		viper.SetConfigFile(f)
		err = viper.ReadInConfig()
		if err != nil {
			t.Fatalf("ReadInConfig err, %v", err)
		}

		err = SetConfig()
		if err != nil {
			t.Fatalf("initConfig err, %v", err)
		}

		cred := Config.Client.Config.Credentials
		if cred.AccessKeyId != "test-ak" {
			t.Fatalf("Credentials.AccessKeyId=%v, wanted: test-ak", cred.AccessKeyId)
		}

		if cred.SecretAccessKey != "test-sk" {
			t.Fatalf("Credentials.SecretAccessKey=%v, wanted: test-sk", cred.SecretAccessKey)
		}
	})
}
