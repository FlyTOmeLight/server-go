package safety

import "regexp"

const (
	forbiddenCommandPattern   = `curl|wget|ping`
	forbiddenIPAddressPattern = `((25[0-5])|(2[0-4]\d)|(1\d\d)|([1-9]\d)|\d)(\.((25[0-5])|(2[0-4]\d)|(1\d\d)|([1-9]\d)|\d)){3}`
	forbiddenDomainPattern    = `[a-zA-Z0-9][-a-zA-Z0-9]{0,62}(\.[a-zA-Z0-9][-a-zA-Z0-9]{0,62})*\.(com|cn|net)`
)

func ValidateCommand(cmd string) bool {
	// 校验命令行规则：
	// 1、不能包含特殊命令字符，curl wget ping
	// 2、不能包含IP地址
	// 3、不能包含域名，其中顶级域名为 .com .cn .net
	if MatchForbiddenCommand(cmd) || MatchForbiddenIPAddress(cmd) || MatchForbiddenDomain(cmd) {
		return false
	}

	return true
}

func MatchForbiddenCommand(cmd string) bool {
	matched, _ := regexp.MatchString(forbiddenCommandPattern, cmd)
	return matched
}

func MatchForbiddenIPAddress(cmd string) bool {
	matched, _ := regexp.MatchString(forbiddenIPAddressPattern, cmd)
	return matched
}

func MatchForbiddenDomain(cmd string) bool {
	matched, _ := regexp.MatchString(forbiddenDomainPattern, cmd)
	return matched
}
