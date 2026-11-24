package election

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"regexp"
	"strings"

	"github.com/FlyTOmeLight/server-go/logger"
)

// 获取k8s namespace的文件默认位置
const DefaultKubernetesNamespaceFile string = "/var/run/secrets/kubernetes.io/serviceaccount/namespace"

// 默认网卡选择正则
var defaultNICPattern = regexp.MustCompile("^(en|em|eth|bond).*")

// LookupLocalIP prefers IPv4 addresses on interfaces matching preferNIC.
// An empty pattern selects common Ethernet and bond interfaces.
func LookupLocalIP(preferNIC string) (string, error) {
	preferNICPattern := defaultNICPattern
	if preferNIC != "" {
		pattern, err := regexp.Compile(preferNIC)
		if err != nil {
			return "", fmt.Errorf("invalid preferred interface pattern: %w", err)
		}
		preferNICPattern = pattern
	}
	// 对于master选主， 获取到是容器内的地址， 所以一般使用默认值就行
	ifaces, err := net.Interfaces()
	if err != nil {
		return "", fmt.Errorf("list network interfaces: %w", err)
	}
	// 遍历所有网卡
	for _, iface := range ifaces {
		// 必须符合正则表达式
		if !preferNICPattern.MatchString(iface.Name) {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			logger.Warnf(context.Background(), "get ip of %s failed, err %v", iface.Name, err)
			continue
		}
		for _, address := range addrs {
			// 检查ip地址判断是否回环地址
			if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
				if ipnet.IP.To4() != nil {
					return ipnet.IP.String(), nil
				}
			}
		}
	}
	logger.Warnf(
		context.Background(),
		"get ip of %s failed, will use first non loopback ip address",
		preferNICPattern.String(),
	)
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "", fmt.Errorf("list interface addresses: %w", err)
	}
	for _, address := range addrs {
		// 检查ip地址判断是否回环地址
		if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String(), nil
			}
		}
	}
	return "", errors.New("no non-loopback IPv4 address found")
}

// GetLocalIP prefers IPv4 addresses on interfaces matching preferNIC, returning
// an empty string on lookup or pattern errors.
func GetLocalIP(preferNIC string) string {
	ip, err := LookupLocalIP(preferNIC)
	if err != nil {
		logger.Warnf(context.Background(), "get local ip failed: %v", err)
		return ""
	}
	return ip
}

// 获取当前程序部署的namespace
func GetCurrentNamespace() (string, error) {
	_, err := os.Stat(DefaultKubernetesNamespaceFile)
	// 文件不存在， 说明不是k8s环境，返回错误
	if err != nil {
		return "", errors.New("not in k8s env")
	}
	// 读取文件，获取namespace
	namespaceBytes, err := os.ReadFile(DefaultKubernetesNamespaceFile)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(namespaceBytes)), nil
}
