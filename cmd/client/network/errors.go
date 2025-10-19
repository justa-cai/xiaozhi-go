package network

import (
	"os"
	"strings"

	"github.com/sirupsen/logrus"
)

// AnalyzeConnectionError 分析连接错误
func AnalyzeConnectionError(err error) {
	logrus.Error("连接错误详细分析:")

	if os.IsTimeout(err) {
		logrus.Error("- 错误类型: 连接超时")
		logrus.Error("- 可能原因: 网络延迟高、服务器无响应或防火墙阻止")
		logrus.Error("- 建议解决方案: 检查网络连接、确认服务器地址正确、检查防火墙设置")
	} else if strings.Contains(err.Error(), "certificate") {
		logrus.Error("- 错误类型: 证书验证错误")
		logrus.Error("- 可能原因: 自签名证书或证书无效")
		logrus.Error("- 建议解决方案: 使用 --skip-tls-verify 选项跳过证书验证")
	} else if strings.Contains(err.Error(), "dial") {
		logrus.Error("- 错误类型: 网络连接错误")
		logrus.Error("- 可能原因: 网络不可达、端口关闭或主机不存在")
		logrus.Error("- 建议解决方案: 确认服务器地址和端口正确、检查网络配置")
	} else if strings.Contains(err.Error(), "proxy") {
		logrus.Error("- 错误类型: 代理连接错误")
		logrus.Error("- 可能原因: 代理配置错误或代理服务不可用")
		logrus.Error("- 建议解决方案: 检查代理配置或暂时禁用代理")
	} else {
		logrus.Error("- 错误类型: 未知错误")
		logrus.Error("- 错误详情:", err.Error())
		logrus.Error("- 建议解决方案: 检查网络环境和服务器状态")
	}
}

// ConnectionError 连接错误类型
type ConnectionError struct {
	Type    string
	Message string
	Cause   error
}

// NewConnectionError 创建新的连接错误
func NewConnectionError(errorType, message string, cause error) *ConnectionError {
	return &ConnectionError{
		Type:    errorType,
		Message: message,
		Cause:   cause,
	}
}

// Error 实现error接口
func (e *ConnectionError) Error() string {
	if e.Cause != nil {
		return e.Message + ": " + e.Cause.Error()
	}
	return e.Message
}

// Unwrap 支持errors.Unwrap
func (e *ConnectionError) Unwrap() error {
	return e.Cause
}

// 预定义的错误类型
var (
	ErrTimeout = NewConnectionError("timeout", "连接超时", nil)
	ErrCertInvalid = NewConnectionError("certificate", "证书验证失败", nil)
	ErrNetworkUnreachable = NewConnectionError("network", "网络不可达", nil)
	ErrProxyFailed = NewConnectionError("proxy", "代理连接失败", nil)
	ErrHandshakeFailed = NewConnectionError("handshake", "握手失败", nil)
	ErrAuthenticationFailed = NewConnectionError("auth", "认证失败", nil)
)