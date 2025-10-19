package network

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/justa-cai/xiaozhi-go/internal/client"
	"github.com/justa-cai/xiaozhi-go/internal/protocol"
	"github.com/sirupsen/logrus"
)

// ConnectionManager WebSocket连接管理器
type ConnectionManager struct {
	protocol        *protocol.WebsocketProtocol
	clientInstance  *client.Client
	config          ConnectionConfig
	handlers        *ConnectionHandlers
}

// ConnectionConfig 连接配置
type ConnectionConfig struct {
	ServerURL       string
	Token           string
	DeviceID        string
	ClientID        string
	SkipTLSVerify   bool
	HandshakeTimeout time.Duration
	HeartbeatInterval time.Duration
}

// ConnectionHandlers 连接处理器
type ConnectionHandlers struct {
	OnConnected      func()
	OnDisconnected   func(error)
	OnJSONMessage    func([]byte)
	OnBinaryMessage  func([]byte)
	OnPingMessage    func()
}

// NewConnectionManager 创建新的连接管理器
func NewConnectionManager(config ConnectionConfig) *ConnectionManager {
	proto := protocol.NewWebsocketProtocol()
	proto.SetSkipTLSVerify(config.SkipTLSVerify)
	proto.SetHandshakeTimeout(config.HandshakeTimeout)

	return &ConnectionManager{
		protocol: proto,
		config:   config,
		handlers: &ConnectionHandlers{},
	}
}

// Connect 连接到服务器
func (cm *ConnectionManager) Connect() error {
	// 设置请求头
	cm.protocol.SetHeader("Authorization", "Bearer "+cm.config.Token)
	cm.protocol.SetHeader("Protocol-Version", "1")
	cm.protocol.SetHeader("Device-Id", cm.config.DeviceID)
	cm.protocol.SetHeader("Client-Id", cm.config.ClientID)

	// 设置协议回调
	cm.setupProtocolCallbacks()

	// 连接服务器
	logrus.Info("准备连接到服务器...")
	err := cm.protocol.Connect(cm.config.ServerURL)
	if err != nil {
		logrus.Errorf("❌ 连接失败: %v", err)
		return err
	}

	return nil
}

// setupProtocolCallbacks 设置协议回调
func (cm *ConnectionManager) setupProtocolCallbacks() {
	// 连接成功回调
	cm.protocol.SetOnConnected(func() {
		logrus.Info("✅ WebSocket连接成功!")

		// 发送hello消息
		helloMsg := map[string]interface{}{
			"type":      "hello",
			"version":   1,
			"transport": "websocket",
			"audio_params": map[string]interface{}{
				"format":         "opus",
				"sample_rate":    16000,
				"channels":       1,
				"frame_duration": 60,
			},
		}

		if err := cm.protocol.SendJSON(helloMsg); err != nil {
			logrus.Errorf("❌ 发送hello消息失败: %v", err)
		} else {
			logrus.Info("✅ hello消息发送成功")
		}

		// 调用用户自定义的连接成功回调
		if cm.handlers.OnConnected != nil {
			cm.handlers.OnConnected()
		}
	})

	// 断开连接回调
	cm.protocol.SetOnDisconnected(func(err error) {
		// 在优雅退出时，不要记录为错误
		if err != nil && !isGracefulShutdown(err) {
			logrus.Errorf("❌ WebSocket断开连接: %v", err)

			// 延迟1秒后尝试重连
			go cm.attemptReconnect()
		} else {
			logrus.Info("WebSocket正常断开连接")
		}

		// 调用用户自定义的断开连接回调
		if cm.handlers.OnDisconnected != nil {
			cm.handlers.OnDisconnected(err)
		}
	})

	// JSON消息回调
	cm.protocol.SetOnJSONMessage(cm.handleJSONMessage)

	// 二进制消息回调
	cm.protocol.SetOnBinaryMessage(cm.handleBinaryMessage)
}

// attemptReconnect 尝试重新连接
func (cm *ConnectionManager) attemptReconnect() {
	logrus.Info("准备在1秒后尝试重新连接...")
	time.Sleep(1 * time.Second)

	logrus.Info("正在尝试重新连接...")
	// 重新设置请求头
	cm.protocol.SetHeader("Authorization", "Bearer "+cm.config.Token)
	cm.protocol.SetHeader("Protocol-Version", "1")
	cm.protocol.SetHeader("Device-Id", cm.config.DeviceID)
	cm.protocol.SetHeader("Client-Id", cm.config.ClientID)

	// 连接
	if err := cm.protocol.Connect(cm.config.ServerURL); err != nil {
		logrus.Errorf("重新连接失败: %v", err)
		AnalyzeConnectionError(err)
	} else {
		logrus.Info("✅ 重新连接成功")
	}
}

// handleJSONMessage 处理JSON消息
func (cm *ConnectionManager) handleJSONMessage(data []byte) {
	// 尝试解析JSON格式以便美观打印
	var jsonData interface{}
	if err := json.Unmarshal(data, &jsonData); err == nil {
		// 简化输出，只显示消息类型
		if typeMap, ok := jsonData.(map[string]interface{}); ok {
			if msgType, exists := typeMap["type"]; exists {
				jsonBytes, _ := json.MarshalIndent(jsonData, "", "  ")
				logrus.Infof("📥 接收到消息类型: %v %s", msgType, string(jsonBytes))

				// 调用用户自定义的JSON消息处理回调
				if cm.handlers.OnJSONMessage != nil {
					cm.handlers.OnJSONMessage(data)
				}
			} else {
				logrus.Info("📥 接收到JSON数据")
				if cm.handlers.OnJSONMessage != nil {
					cm.handlers.OnJSONMessage(data)
				}
			}
		}
	} else {
		logrus.Infof("📥 接收到文本数据: %s", string(data))
		if cm.handlers.OnJSONMessage != nil {
			cm.handlers.OnJSONMessage(data)
		}
	}
}

// handleBinaryMessage 处理二进制消息
func (cm *ConnectionManager) handleBinaryMessage(data []byte) {
	// 调用用户自定义的二进制消息处理回调
	if cm.handlers.OnBinaryMessage != nil {
		cm.handlers.OnBinaryMessage(data)
	}
}

// SetHandlers 设置连接处理器
func (cm *ConnectionManager) SetHandlers(handlers *ConnectionHandlers) {
	cm.handlers = handlers
}

// SetClient 设置客户端实例
func (cm *ConnectionManager) SetClient(clientInstance *client.Client) {
	cm.clientInstance = clientInstance
}

// IsConnected 检查是否已连接
func (cm *ConnectionManager) IsConnected() bool {
	return cm.protocol.IsConnected()
}

// SendJSON 发送JSON消息
func (cm *ConnectionManager) SendJSON(data interface{}) error {
	return cm.protocol.SendJSON(data)
}

// SendPing 发送心跳包
func (cm *ConnectionManager) SendPing() error {
	pingMsg := map[string]interface{}{
		"type": "ping",
		"id":   time.Now().Unix(),
	}

	return cm.protocol.SendJSON(pingMsg)
}

// isGracefulShutdown 判断是否为优雅关闭
func isGracefulShutdown(err error) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()
	return strings.Contains(errStr, "use of closed network connection") ||
		   strings.Contains(errStr, "connection reset by peer") ||
		   strings.Contains(errStr, "EOF") ||
		   strings.Contains(errStr, "i/o timeout") ||
		   strings.Contains(errStr, "timeout")
}

// ForceDisconnect 强制断开连接
func (cm *ConnectionManager) ForceDisconnect() {
	if cm.protocol != nil {
		// 设置很短的读取超时，让ReadMessage立即返回
		cm.protocol.SetReadTimeout(1 * time.Millisecond)
		cm.protocol.ForceDisconnect()
	}
}

// Close 关闭连接
func (cm *ConnectionManager) Close() {
	if cm.protocol != nil {
		cm.protocol.ForceDisconnect()
	}
}

// GetProtocol 获取协议实例
func (cm *ConnectionManager) GetProtocol() protocol.Protocol {
	return cm.protocol
}

// StartHeartbeat 启动心跳定时器
func (cm *ConnectionManager) StartHeartbeat() {
	ticker := time.NewTicker(cm.config.HeartbeatInterval)
	go func() {
		for range ticker.C {
			if cm.IsConnected() {
				if err := cm.SendPing(); err != nil {
					logrus.Warnf("发送心跳包失败: %v", err)
				}

				// 调用用户自定义的Ping消息回调
				if cm.handlers.OnPingMessage != nil {
					cm.handlers.OnPingMessage()
				}
			}
		}
	}()
}