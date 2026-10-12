package conf

import (
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func withStubDecryptor(t *testing.T, decryptor func(ciphertext string) (string, error)) {
	t.Helper()
	secretDecryptorMu.RLock()
	origin := secretDecryptor
	secretDecryptorMu.RUnlock()
	SetSecretDecryptor(decryptor)
	t.Cleanup(func() {
		SetSecretDecryptor(origin)
	})
}

func TestResolveValueSecret(t *testing.T) {
	withStubDecryptor(t, func(ciphertext string) (string, error) {
		return "plain(" + ciphertext + ")", nil
	})

	tests := []struct {
		name     string
		text     string
		expected string
	}{
		{name: "no placeholder", text: "hello world", expected: "hello world"},
		{name: "whole secret", text: "${secret: abc}", expected: "plain(abc)"},
		{name: "secret without space", text: "${secret:abc}", expected: "plain(abc)"},
		{name: "embedded secret", text: "prefix-${secret: abc}-suffix", expected: "prefix-plain(abc)-suffix"},
		{name: "multiple secrets", text: "${secret: a}/${secret: b}", expected: "plain(a)/plain(b)"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ResolveValue(tt.text)
			assert.Nil(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestResolveValueSecretError(t *testing.T) {
	withStubDecryptor(t, func(ciphertext string) (string, error) {
		return "", errors.New("decrypt failed")
	})

	_, err := ResolveValue("${secret: abc}")
	assert.NotNil(t, err)
}

func TestResolveValueEnv(t *testing.T) {
	t.Setenv("TEST_CONF_ENV", "from-env")

	tests := []struct {
		name     string
		text     string
		expected string
		hasErr   bool
	}{
		{name: "env set", text: "${env: TEST_CONF_ENV}", expected: "from-env"},
		{name: "env set ignores default", text: "${env: TEST_CONF_ENV:123}", expected: "from-env"},
		{name: "env missing with default", text: "${env: TEST_CONF_ENV_MISSING:123}", expected: "123"},
		{name: "env missing without default", text: "${env: TEST_CONF_ENV_MISSING}", hasErr: true},
		{name: "embedded env", text: "prefix-${env: TEST_CONF_ENV}-suffix", expected: "prefix-from-env-suffix"},
		{name: "default contains colon", text: "${env: TEST_CONF_ENV_MISSING:a:b}", expected: "a:b"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ResolveValue(tt.text)
			if tt.hasErr {
				assert.NotNil(t, err)
				return
			}
			assert.Nil(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestResolveValueFile(t *testing.T) {
	filePath := filepath.Join(t.TempDir(), "secret.txt")
	assert.Nil(t, os.WriteFile(filePath, []byte("file-content\n"), 0o600))

	tests := []struct {
		name     string
		text     string
		expected string
		hasErr   bool
	}{
		{name: "file exists", text: "${file: " + filePath + "}", expected: "file-content"},
		{name: "file missing", text: "${file: /not/exist/path}", hasErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ResolveValue(tt.text)
			if tt.hasErr {
				assert.NotNil(t, err)
				return
			}
			assert.Nil(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestResolveValueCaseInsensitive(t *testing.T) {
	withStubDecryptor(t, func(ciphertext string) (string, error) {
		return "plain(" + ciphertext + ")", nil
	})
	t.Setenv("TEST_CONF_ENV", "from-env")

	filePath := filepath.Join(t.TempDir(), "secret.txt")
	assert.Nil(t, os.WriteFile(filePath, []byte("file-content"), 0o600))

	tests := []struct {
		name     string
		text     string
		expected string
	}{
		{name: "upper secret", text: "${SECRET: abc}", expected: "plain(abc)"},
		{name: "mixed env", text: "${Env: TEST_CONF_ENV}", expected: "from-env"},
		{name: "upper env with default", text: "${ENV: TEST_CONF_ENV_MISSING:123}", expected: "123"},
		{name: "upper file", text: "${FILE: " + filePath + "}", expected: "file-content"},
		{name: "mixed file", text: "${File: " + filePath + "}", expected: "file-content"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ResolveValue(tt.text)
			assert.Nil(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestEncryptSecretRoundTrip(t *testing.T) {
	t.Setenv(EnvSecretKey, "0123456789abcdef0123456789abcdef")
	for _, text := range []string{"", "my-password", "中文密文"} {
		first, err := EncryptSecret(text)
		if !assert.NoError(t, err) {
			return
		}
		second, err := EncryptSecret(text)
		assert.NoError(t, err)
		assert.NotEqual(t, first, second, "every encryption requires a fresh nonce")
		assert.True(t, strings.HasPrefix(first, secretFormat))
		plaintext, err := defaultSecretDecryptor(first)
		assert.NoError(t, err)
		assert.Equal(t, text, plaintext)
		resolved, err := ResolveValue("${secret: " + first + "}")
		assert.NoError(t, err)
		assert.Equal(t, text, resolved)
	}
}

func TestSecretRequiresExplicitKey(t *testing.T) {
	for _, key := range []string{"", "short"} {
		t.Setenv(EnvSecretKey, key)
		_, err := EncryptSecret("password")
		assert.ErrorContains(t, err, EnvSecretKey)
		_, err = defaultSecretDecryptor("v1:AAAA")
		assert.ErrorContains(t, err, EnvSecretKey)
	}
}

func TestSecretRejectsMalformedCiphertext(t *testing.T) {
	t.Setenv(EnvSecretKey, "0123456789abcdef")
	valid, err := EncryptSecret("password")
	if !assert.NoError(t, err) {
		return
	}
	data, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(valid, secretFormat))
	assert.NoError(t, err)
	data[len(data)-1] ^= 1
	tampered := secretFormat + base64.StdEncoding.EncodeToString(data)
	for _, ciphertext := range []string{"", "qg5lFlIxLWKUtXUKi9FVoLfmiTNDTRVjQfOh7k/ti+o=", "v2:AAAA", "v1:!", "v1:", "v1:YQ==", tampered} {
		t.Run(ciphertext, func(t *testing.T) {
			_, err := defaultSecretDecryptor(ciphertext)
			assert.Error(t, err)
		})
	}
	t.Setenv(EnvSecretKey, "fedcba9876543210")
	_, err = defaultSecretDecryptor(valid)
	assert.ErrorContains(t, err, "authentication failed")
}

func TestUnmarshalKeyWithPlaceholder(t *testing.T) {
	withStubDecryptor(t, func(ciphertext string) (string, error) {
		return "decrypted-" + ciphertext, nil
	})
	t.Setenv("TEST_CONF_USER", "root")

	type testConfig struct {
		User     string
		Password string
	}

	viper.Set("TestPlaceholder", map[string]any{
		"User":     "${env: TEST_CONF_USER}",
		"Password": "${secret: cipher-text}",
	})
	defer viper.Set("TestPlaceholder", nil)

	var config testConfig
	snapshot, err := FromSettings(map[string]any{"AppName": "test", "AppCode": 100, "RunMode": "release", "TestPlaceholder": viper.Get("TestPlaceholder")})
	require.NoError(t, err)
	err = snapshot.UnmarshalKey("TestPlaceholder", &config)
	assert.Nil(t, err)
	assert.Equal(t, "root", config.User)
	assert.Equal(t, "decrypted-cipher-text", config.Password)
}
