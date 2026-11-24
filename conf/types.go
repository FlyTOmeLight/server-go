package conf

import (
	"errors"
	"fmt"
	"strings"
)

const (
	// RunModeDebug 调试
	RunModeDebug = "debug"

	// RunModeTest 测试
	RunModeTest = "test"

	// RunModeRelease 线上发布
	RunModeRelease = "release"
)

const (
	DeployModeCloud DeployMode = "Cloud"
	DeployModeEdge  DeployMode = "Edge"
)

type DeployMode string

// IsCloud 判断是否为云模式
func (dm DeployMode) IsCloud() bool {
	return dm == DeployModeCloud || dm == ""
}

// IsEdge 判断是否为边缘计算模式
func (dm DeployMode) IsEdge() bool {
	return dm == DeployModeEdge
}

const (
	AuthTypeFake       AuthType = "Fake"
	AuthTypeAuthCenter AuthType = "AuthCenter"
)

type AuthType string

type Config struct {
	AppName    string
	AppCode    int
	RunMode    string
	Feature    any
	DeployMode DeployMode

	Services   []string
	HTTPServer HTTPServerConfig
	FakeAuth   FakeAuthConfig

	settings map[string]any
}

// 检查配置文件是否合规
func (c *Config) check() error {
	if c.AppName == "" {
		return errors.New("Config.AppName is empty")
	}
	if c.AppCode == 0 {
		return errors.New("Config.AppCode is empty")
	}
	if c.RunMode != RunModeDebug && c.RunMode != RunModeTest && c.RunMode != RunModeRelease {
		return errors.New("Config.RunMode must be debug, test, or release")
	}
	http := c.HTTPServer
	for _, field := range []struct {
		name  string
		value int
	}{
		{name: "ReadTimeout", value: http.ReadTimeout},
		{name: "WriteTimeout", value: http.WriteTimeout},
		{name: "IdleTimeout", value: http.IdleTimeout},
		{name: "ReadHeaderTimeout", value: http.ReadHeaderTimeout},
		{name: "PreStopDelay", value: http.PreStopDelay},
		{name: "MaxHeaderBytes", value: http.MaxHeaderBytes},
	} {
		if field.value < 0 {
			return fmt.Errorf("Config.HTTPServer.%s must be non-negative", field.name)
		}
	}
	if http.TLS.Enable {
		if strings.TrimSpace(http.TLS.CertFile) == "" {
			return errors.New("Config.HTTPServer.TLS.CertFile is required when TLS is enabled")
		}
		if strings.TrimSpace(http.TLS.KeyFile) == "" {
			return errors.New("Config.HTTPServer.TLS.KeyFile is required when TLS is enabled")
		}
	}
	if http.AdminListen != "" && http.AdminListen == http.Listen && !strings.HasSuffix(http.Listen, ":0") {
		return errors.New("Config.HTTPServer.AdminListen must use a different address from Listen")
	}

	c.setDefaults()
	return nil
}

func (c *Config) setDefaults() {
	if c.DeployMode == "" {
		c.DeployMode = defaultDeployMode
	}
	if c.HTTPServer.AuthType == "" && c.RunMode != RunModeRelease {
		c.HTTPServer.AuthType = AuthTypeFake
	}
}

type FakeAuthConfig struct {
	TenantID string
	UserID   string
	UserName string
	UserRole string
}

type HTTPServerConfig struct {
	Listen            string
	ReadTimeout       int // 单位 ms
	WriteTimeout      int // 单位 ms
	IdleTimeout       int
	ReadHeaderTimeout int
	PreStopDelay      int
	MaxHeaderBytes    int
	TrustedProxies    []string
	AdminListen       string
	TLS               HTTPServerTLSConfig
	APIBasePath       string
	PprofEnable       bool
	MetricsEnable     bool
	SwaggerEnable     bool
	Trace             HTTPServerTraceConfig
	UseH2C            bool // 支持HTTP2 无SSL
	AuthType          AuthType

	// ConsoleEnable 是否开启Console API反向代理功能
	// 该反向代理会将前端API请求（路径包含/api）转换为内部OpenAPI请求（路径包含/internal）
	// 例如：POST /example/api/v1/GetProject => POST /example/internal/v1/GetProject
	ConsoleEnable bool

	ConsoleProxyNames []string

	// AltoHandlerAPIError 是否开启针对 /{appName}/api/* 接口的自动状态码处理功能
	// 与ConsoleEnable互斥，仅当ConsoleEnable=false时生效
	// 当开启时，会拦截所有 /{appName}/api/* 接口的响应：
	//   - 将所有非200的HTTP状态码统一改为200，但保持响应体内容不变
	//   - 对于 errors.Error 类型的错误，会规范化处理，自动设置 RequestID 并确保 StatusCode 字段正确设置
	AltoHandlerAPIError bool

	Assets []HTTPServerAssetConfig

	// 开启针对 /{appName}/api/* 接口的自动状态码处理功能 （白名单）
	AllowAPIPathNoHandlerAPIError []string
}

type HTTPServerTLSConfig struct {
	Enable       bool
	CertFile     string
	KeyFile      string
	ClientCAFile string
}

type HTTPServerAssetConfig struct {
	AuthEnable bool
	URLPath    string
	FilePath   string
}

type HTTPServerTraceConfig struct {
	PrefixFilters []string
	SuffixFilters []string
}
