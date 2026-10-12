package conf

import (
	"fmt"
	"os"
	"reflect"
	"regexp"
	"strings"

	"github.com/go-viper/mapstructure/v2"
	"github.com/spf13/viper"

	"github.com/FlyTOmeLight/server-go/pkg/safety"
)

const (
	// EnvSecretKey 解密 ${secret: xxx} 密文使用的 AES 密钥环境变量名
	EnvSecretKey = "SERVER_SECRET_KEY"

	// EnvSecretIV 解密 ${secret: xxx} 密文使用的 AES IV 环境变量名
	EnvSecretIV = "SERVER_SECRET_IV"
)

const (
	// defaultSecretKey 未设置 SERVER_SECRET_KEY 时使用的默认 AES 密钥
	defaultSecretKey = "6kY8uyQEl6XV37KR"

	// defaultSecretIV 未设置 SERVER_SECRET_IV 时使用的默认 AES IV
	defaultSecretIV = "E5ZKizMERreD3HX8"
)

// valuePattern 匹配以下形式的占位符（secret/env/file 关键字不区分大小写）：
//   - ${secret: xxx}         密文，自动解密
//   - ${env: NAME[:default]} 环境变量，可带默认值
//   - ${file: /path}         文件内容
var valuePattern = regexp.MustCompile(`\$\{(?i:(secret|env|file)):([^{}]+?)\}`)

// secretDecryptor 解密 ${secret: xxx} 中的密文，默认使用 AES-CBC（密钥/IV 取自环境变量），可通过 SetSecretDecryptor 替换
var secretDecryptor = defaultSecretDecryptor

// SetSecretDecryptor 替换默认的密文解密实现
func SetSecretDecryptor(decryptor func(ciphertext string) (string, error)) {
	if decryptor != nil {
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

// UnmarshalKey 等同于 viper.UnmarshalKey，但会自动解析 ${secret: xxx}、${env: xxx}、${file: xxx} 占位符
func UnmarshalKey(key string, rawVal any) error {
	return viper.UnmarshalKey(key, rawVal, DecodeHook())
}

// Unmarshal 等同于 viper.Unmarshal，但会自动解析 ${secret: xxx}、${env: xxx}、${file: xxx} 占位符
func Unmarshal(rawVal any) error {
	return viper.Unmarshal(rawVal, DecodeHook())
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

// resolvePlaceholder 按占位符类型解析实际值，kind 不区分大小写
func resolvePlaceholder(kind, spec string) (string, error) {
	switch strings.ToLower(kind) {
	case "secret":
		plaintext, err := secretDecryptor(strings.TrimSpace(spec))
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

// defaultSecretDecryptor 使用 AES-CBC 解密，密钥与 IV 分别取自 SERVER_SECRET_KEY / SERVER_SECRET_IV 环境变量，未设置时使用内置默认值
func defaultSecretDecryptor(ciphertext string) (string, error) {
	key, iv := secretKeyIV()
	return safety.CBCDecrypter(ciphertext, []byte(key), []byte(iv))
}

// EncryptSecret 使用与 defaultSecretDecryptor 相同的密钥/IV 加密明文，用于生成 ${secret: xxx} 密文
func EncryptSecret(plaintext string) (string, error) {
	key, iv := secretKeyIV()
	return safety.CBCEncrypter(plaintext, []byte(key), []byte(iv))
}

// secretKeyIV 返回 AES 密钥与 IV，分别取自 SERVER_SECRET_KEY / SERVER_SECRET_IV 环境变量，未设置时使用内置默认值
func secretKeyIV() (string, string) {
	key := os.Getenv(EnvSecretKey)
	if key == "" {
		key = defaultSecretKey
	}
	iv := os.Getenv(EnvSecretIV)
	if iv == "" {
		iv = defaultSecretIV
	}
	return key, iv
}
