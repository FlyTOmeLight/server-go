package election

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"regexp"

	"github.com/FlyTOmeLight/server-go/logger"
)

// 获取k8s namespace的文件默认位置
const DefaultKubernetesNamespaceFile string = "/var/run/secrets/kubernetes.io/serviceaccount/namespace"

// 默认网卡选择正则
var defaultNICPattern *regexp.Regexp = regexp.MustCompile("en.*|em.*|eth.*|bond.*")

// 获取本机IP地址
// preferNIC是默认获取到网卡名称，是一个正则表达式
// 不传递时默认的 prreferNIC为 en.*|em.*|eth.* , 有多个网卡满足条件时，会返回字典序的第一个
func GetLocalIP(preferNIC string) string {
	// 匹配网卡名称的正则表达式
	preferNICPattern := defaultNICPattern
	if preferNIC != "" {
		preferNICPattern = regexp.MustCompile(preferNIC)
	}
	// 对于master选主， 获取到是容器内的地址， 所以一般使用默认值就行
	ifaces, err := net.Interfaces()
	if err != nil {
		fmt.Println(err)
		return ""
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
					return ipnet.IP.String()
				}
			}
		}
	}
	logger.Warnf(context.Background(), "get ip of %s failed, will use first non loopback ip address", preferNICPattern.String())
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		logger.Warnf(context.Background(), "get ip failed, err %v", err)
		return ""
	}
	for _, address := range addrs {
		// 检查ip地址判断是否回环地址
		if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String()
			}
		}
	}
	logger.Warnf(context.Background(), "no non loopback ip address found")
	return ""
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
	return string(namespaceBytes), nil
}
