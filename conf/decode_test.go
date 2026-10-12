package conf

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func withStubDecryptor(t *testing.T, decryptor func(ciphertext string) (string, error)) {
	t.Helper()
	origin := secretDecryptor
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

func TestDefaultSecretDecryptor(t *testing.T) {
	t.Run("missing env uses default key", func(t *testing.T) {
		t.Setenv(EnvSecretKey, "")
		t.Setenv(EnvSecretIV, "")

		plaintext, err := defaultSecretDecryptor("qg5lFlIxLWKUtXUKi9FVoLfmiTNDTRVjQfOh7k/ti+o=")
		assert.Nil(t, err)
		assert.Equal(t, "oooooo11111113442", plaintext)
	})

	t.Run("aes cbc", func(t *testing.T) {
		t.Setenv(EnvSecretKey, "6kY8uyQEl6XV37KR")
		t.Setenv(EnvSecretIV, "E5ZKizMERreD3HX8")

		plaintext, err := defaultSecretDecryptor("qg5lFlIxLWKUtXUKi9FVoLfmiTNDTRVjQfOh7k/ti+o=")
		assert.Nil(t, err)
		assert.Equal(t, "oooooo11111113442", plaintext)
	})
}

func TestEncryptSecretRoundTrip(t *testing.T) {
	t.Setenv(EnvSecretKey, "")
	t.Setenv(EnvSecretIV, "")

	ciphertext, err := EncryptSecret("my-password")
	assert.Nil(t, err)

	plaintext, err := defaultSecretDecryptor(ciphertext)
	assert.Nil(t, err)
	assert.Equal(t, "my-password", plaintext)
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
	err := UnmarshalKey("TestPlaceholder", &config)
	assert.Nil(t, err)
	assert.Equal(t, "root", config.User)
	assert.Equal(t, "decrypted-cipher-text", config.Password)
}
