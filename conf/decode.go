package conf

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"reflect"
	"regexp"
	"strings"
	"sync"

	"github.com/go-viper/mapstructure/v2"
	"github.com/spf13/viper"
)

// EnvSecretKey is the explicit AES key used for configuration secrets.
const EnvSecretKey = "SERVER_SECRET_KEY"

// secretFormat identifies the authenticated ciphertext format.
const secretFormat = "v1:"

// valuePattern 匹配以下形式的占位符（secret/env/file 关键字不区分大小写）：
//   - ${secret: xxx}         密文，自动解密
//   - ${env: NAME[:default]} 环境变量，可带默认值
//   - ${file: /path}         文件内容
var valuePattern = regexp.MustCompile(`\$\{(?i:(secret|env|file)):([^{}]+?)\}`)

// secretDecryptor 解密 ${secret: xxx} 中的密文，默认使用 AES-GCM，可通过 SetSecretDecryptor 替换
var (
	secretDecryptor   = defaultSecretDecryptor
	secretDecryptorMu sync.RWMutex
)

// SetSecretDecryptor 替换默认的密文解密实现
func SetSecretDecryptor(decryptor func(ciphertext string) (string, error)) {
	if decryptor != nil {
		secretDecryptorMu.Lock()
		defer secretDecryptorMu.Unlock()
		secretDecryptor = decryptor
	}
}

// DecodeHook 返回支持 ${secret: xxx}、${env: xxx}、${file: xxx} 占位符自动解析的
// viper.DecoderConfigOption，同时保留 viper 默认的 time.Duration 与字符串切片转换 Hook
func DecodeHook() viper.DecoderConfigOption {
	return viper.DecodeHook(mapstructure.ComposeDecodeHookFunc(
		ValueDecodeHookFunc(),
		mapstructure.StringToTimeDurationHookFunc(),
		mapstructure.StringToSliceHookFunc(","),
	))
}

// ValueDecodeHookFunc 返回 mapstructure.DecodeHookFunc，
// 当字符串字段中出现 ${secret: xxx}、${env: xxx}、${file: xxx} 占位符时自动解析为实际值
func ValueDecodeHookFunc() mapstructure.DecodeHookFunc {
	return func(from reflect.Type, to reflect.Type, data any) (any, error) {
		if from.Kind() != reflect.String || to.Kind() != reflect.String {
			return data, nil
		}
		return ResolveValue(data.(string))
	}
}

// ResolveValue 将文本中所有 ${secret: xxx}、${env: xxx}、${file: xxx} 占位符替换为解析后的实际值
func ResolveValue(text string) (string, error) {
	if !valuePattern.MatchString(text) {
		return text, nil
	}

	var resolveErr error
	result := valuePattern.ReplaceAllStringFunc(text, func(match string) string {
		if resolveErr != nil {
			return match
		}
		submatch := valuePattern.FindStringSubmatch(match)
		resolved, err := resolvePlaceholder(submatch[1], submatch[2])
		if err != nil {
			resolveErr = err
			return match
		}
		return resolved
	})
	return result, resolveErr
}

func resolveSettings(settings map[string]any) (map[string]any, error) {
	resolved := make(map[string]any, len(settings))
	for key, value := range settings {
		next, err := resolveSetting(value)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", key, err)
		}
		resolved[key] = next
	}
	return resolved, nil
}

func resolveSetting(value any) (any, error) {
	data := reflect.ValueOf(value)
	if !data.IsValid() {
		return value, nil
	}
	switch data.Kind() {
	case reflect.String:
		return ResolveValue(data.String())
	case reflect.Map:
		if data.Type().Key().Kind() != reflect.String {
			return value, nil
		}
		settings := make(map[string]any, data.Len())
		for entries := data.MapRange(); entries.Next(); {
			settings[entries.Key().String()] = entries.Value().Interface()
		}
		return resolveSettings(settings)
	case reflect.Slice, reflect.Array:
		resolved := make([]any, data.Len())
		for index := range resolved {
			next, err := resolveSetting(data.Index(index).Interface())
			if err != nil {
				return nil, fmt.Errorf("item %d: %w", index, err)
			}
			resolved[index] = next
		}
		return resolved, nil
	default:
		return value, nil
	}
}

// resolvePlaceholder 按占位符类型解析实际值，kind 不区分大小写
func resolvePlaceholder(kind, spec string) (string, error) {
	switch strings.ToLower(kind) {
	case "secret":
		secretDecryptorMu.RLock()
		decryptor := secretDecryptor
		secretDecryptorMu.RUnlock()
		plaintext, err := decryptor(strings.TrimSpace(spec))
		if err != nil {
			return "", fmt.Errorf("DecryptSecretError: %v", err)
		}
		return plaintext, nil
	case "env":
		name, def, hasDef := splitSpec(spec)
		if value, ok := os.LookupEnv(name); ok {
			return value, nil
		}
		if hasDef {
			return def, nil
		}
		return "", fmt.Errorf("environment variable %s is not set and no default value provided", name)
	case "file":
		path := strings.TrimSpace(spec)
		content, err := os.ReadFile(path)
		if err != nil {
			return "", fmt.Errorf("ReadFileError path: %s, err: %v", path, err)
		}
		// 去除文件末尾的换行符，兼容密钥文件以换行结尾的常见写法
		return strings.TrimRight(string(content), "\r\n"), nil
	default:
		return "", fmt.Errorf("unsupported placeholder type: %s", kind)
	}
}

// splitSpec 按第一个冒号拆分占位符内容为 目标 与 默认值（可选），目标去除首尾空白，默认值保持原样
func splitSpec(spec string) (target, def string, hasDef bool) {
	target, def, hasDef = strings.Cut(spec, ":")
	return strings.TrimSpace(target), def, hasDef
}

// defaultSecretDecryptor accepts only the versioned AES-GCM format.
func defaultSecretDecryptor(ciphertext string) (string, error) {
	if !strings.HasPrefix(ciphertext, secretFormat) {
		return "", fmt.Errorf(
			"unsupported secret format: expected %s AES-GCM ciphertext; re-encrypt legacy secrets",
			secretFormat,
		)
	}
	aead, err := secretCipher()
	if err != nil {
		return "", err
	}
	data, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(ciphertext, secretFormat))
	if err != nil {
		return "", fmt.Errorf("invalid secret encoding: %w", err)
	}
	if len(data) < aead.NonceSize()+aead.Overhead() {
		return "", errors.New("secret ciphertext is too short")
	}
	plaintext, err := aead.Open(nil, data[:aead.NonceSize()], data[aead.NonceSize():], []byte(secretFormat))
	if err != nil {
		return "", fmt.Errorf("secret authentication failed: %w", err)
	}
	return string(plaintext), nil
}

// EncryptSecret encrypts a configuration value using a fresh random nonce.
// SERVER_SECRET_KEY must explicitly contain a 16, 24, or 32 byte AES key.
func EncryptSecret(plaintext string) (string, error) {
	aead, err := secretCipher()
	if err != nil {
		return "", err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("generate secret nonce: %w", err)
	}
	data := aead.Seal(nonce, nonce, []byte(plaintext), []byte(secretFormat))
	return secretFormat + base64.StdEncoding.EncodeToString(data), nil
}

func secretCipher() (cipher.AEAD, error) {
	key := os.Getenv(EnvSecretKey)
	if key == "" {
		return nil, fmt.Errorf("%s must be set to a 16, 24, or 32 byte AES key", EnvSecretKey)
	}
	block, err := aes.NewCipher([]byte(key))
	if err != nil {
		return nil, fmt.Errorf("invalid %s: %w", EnvSecretKey, err)
	}
	return cipher.NewGCM(block)
}
