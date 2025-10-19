package client

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/justa-cai/xiaozhi-go/internal/protocol"
	"github.com/sirupsen/logrus"
)

// 客户端状态常量
const (
	StateIdle       = "idle"       // 空闲状态
	StateConnecting = "connecting" // 正在连接状态
	StateListening  = "listening"  // 监听状态（录音中）
	StateSpeaking   = "speaking"   // 播放状态（播放TTS）
)

// 监听模式常量
const (
	ListenModeAuto     = "auto"     // 自动模式
	ListenModeManual   = "manual"   // 手动模式
	ListenModeRealtime = "realtime" // 实时模式
)

// AudioChannel 配置
const (
	DefaultWebSocketURL      = "wss://api.tenclass.net/xiaozhi/v1/"
	DefaultHelloTimeout      = 10 * time.Second
	DefaultOpusFrameDuration = 60 // 毫秒
)

// Client 定义小智客户端结构
type Client struct {
	// 协议实现
	protocol protocol.Protocol

	// 状态控制
	mu         sync.Mutex
	state      string
	sessionID  string
	deviceID   string
	clientID   string
	token      string
	listenMode string

	// 事件回调
	onStateChanged       func(oldState, newState string)
	onNetworkError       func(err error)
	onRecognizedText     func(text string)
	onSpeakText          func(text string)
	onAudioData          func(data []byte)
	onEmotionChanged     func(emotion, text string)
	onIoTCommand         func(commands []interface{})
	onAudioChannelOpen   func()
	onAudioChannelClosed func()

	// 内部控制
	helloReceived chan struct{}
}

// New 创建一个新的客户端实例
func New(protocol protocol.Protocol) *Client {
	client := &Client{
		protocol:      protocol,
		state:         StateIdle,
		helloReceived: make(chan struct{}),
	}

	// 设置协议回调
	protocol.SetOnJSONMessage(client.handleJSONMessage)
	protocol.SetOnBinaryMessage(client.handleBinaryMessage)
	protocol.SetOnDisconnected(client.handleDisconnected)
	protocol.SetOnConnected(client.handleConnected)

	return client
}

// SetDeviceID 设置设备ID
func (c *Client) SetDeviceID(deviceID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.deviceID = deviceID
}

// SetClientID 设置客户端ID
func (c *Client) SetClientID(clientID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.clientID = clientID
}

// SetToken 设置访问令牌
func (c *Client) SetToken(token string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.token = token
}

// SetOnStateChanged 设置状态变更的回调
func (c *Client) SetOnStateChanged(callback func(oldState, newState string)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.onStateChanged = callback
}

// SetOnNetworkError 设置网络错误的回调
func (c *Client) SetOnNetworkError(callback func(err error)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.onNetworkError = callback
}

// SetOnRecognizedText 设置识别文本的回调
func (c *Client) SetOnRecognizedText(callback func(text string)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.onRecognizedText = callback
}

// SetOnSpeakText 设置朗读文本的回调
func (c *Client) SetOnSpeakText(callback func(text string)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.onSpeakText = callback
}

// SetOnAudioData 设置音频数据的回调
func (c *Client) SetOnAudioData(callback func(data []byte)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.onAudioData = callback
}

// SetOnEmotionChanged 设置情感变更的回调
func (c *Client) SetOnEmotionChanged(callback func(emotion, text string)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.onEmotionChanged = callback
}

// SetOnIoTCommand 设置IoT命令的回调
func (c *Client) SetOnIoTCommand(callback func(commands []interface{})) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.onIoTCommand = callback
}

// SetOnAudioChannelOpen 设置音频通道打开的回调
func (c *Client) SetOnAudioChannelOpen(callback func()) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.onAudioChannelOpen = callback
}

// SetOnAudioChannelClosed 设置音频通道关闭的回调
func (c *Client) SetOnAudioChannelClosed(callback func()) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.onAudioChannelClosed = callback
}

// GetState 获取当前状态
func (c *Client) GetState() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.state
}

// SetState 内部方法，用于更新状态并触发回调
func (c *Client) SetState(newState string) {
	c.mu.Lock()
	oldState := c.state
	c.state = newState
	onStateChanged := c.onStateChanged
	c.mu.Unlock()

	if oldState != newState && onStateChanged != nil {
		onStateChanged(oldState, newState)
	}
}

// OpenAudioChannel 打开音频通道
func (c *Client) OpenAudioChannel(url string) error {
	c.mu.Lock()
	if c.state != StateIdle {
		c.mu.Unlock()
		return errors.New("客户端不在空闲状态，无法打开音频通道")
	}
	c.SetState(StateConnecting)

	// 准备请求头 - 确保请求头设置完整
	if c.token != "" {
		c.protocol.SetHeader("Authorization", fmt.Sprintf("Bearer %s", c.token))
		logrus.Debugf("设置Authorization头: %s", fmt.Sprintf("Bearer %s", c.token))
	}
	c.protocol.SetHeader("Protocol-Version", "1")
	logrus.Debug("设置Protocol-Version头: 1")

	if c.deviceID != "" {
		c.protocol.SetHeader("Device-Id", c.deviceID)
		logrus.Debugf("设置Device-Id头: %s", c.deviceID)
	} else {
		// 尝试获取MAC地址作为设备ID
		interfaces, err := net.Interfaces()
		if err == nil {
			for _, i := range interfaces {
				if i.HardwareAddr != nil && len(i.HardwareAddr) > 0 {
					c.deviceID = i.HardwareAddr.String()
					c.protocol.SetHeader("Device-Id", c.deviceID)
					logrus.Debugf("设置Device-Id头(MAC): %s", c.deviceID)
					break
				}
			}
		}
	}

	if c.clientID != "" {
		c.protocol.SetHeader("Client-Id", c.clientID)
		logrus.Debugf("设置Client-Id头: %s", c.clientID)
	} else {
		// 生成UUID作为客户端ID
		c.clientID = uuid.New().String()
		c.protocol.SetHeader("Client-Id", c.clientID)
		logrus.Debugf("设置Client-Id头(新生成): %s", c.clientID)
	}

	// 打印请求头和WebSocket地址
	headers := c.protocol.GetHeaders()
	logrus.Infof("WebSocket请求头: %v", headers)

	// 重置hello接收通道
	c.helloReceived = make(chan struct{})
	c.mu.Unlock()

	// 如果URL为空，使用默认URL
	if url == "" {
		url = DefaultWebSocketURL
	}

	// 打印WebSocket地址
	logrus.Infof("WebSocket地址: %s", url)

	// 连接WebSocket服务器
	var err error

	// 使用更短的连接超时，与测试模式保持一致
	connectDone := make(chan error, 1)
	go func() {
		logrus.Debug("开始尝试WebSocket连接...")
		connectStart := time.Now()
		connErr := c.protocol.Connect(url)
		elapsed := time.Since(connectStart)
		logrus.Debugf("WebSocket连接尝试完成，耗时: %v, 结果: %v", elapsed, connErr)
		connectDone <- connErr
	}()

	// 更短的连接超时 (15秒)
	select {
	case err = <-connectDone:
		if err != nil {
			logrus.Errorf("WebSocket连接失败: %v", err)
			c.SetState(StateIdle)
			return err
		}
		logrus.Info("WebSocket连接成功，准备发送hello消息")
	case <-time.After(15 * time.Second):
		logrus.Error("WebSocket连接超时 (15秒)")
		c.SetState(StateIdle)
		return errors.New("连接WebSocket服务器超时")
	}

	// 发送Hello消息
	hello := protocol.HelloMessage{
		Type:      "hello",
		Version:   1,
		Transport: "websocket",
		AudioParams: protocol.AudioParams{
			Format:        "opus",
			SampleRate:    16000,
			Channels:      1,
			FrameDuration: DefaultOpusFrameDuration,
		},
	}

	// 发送hello前记录日志
	logJSON, _ := json.Marshal(hello)
	logrus.Debugf("发送hello消息: %s", string(logJSON))

	// 发送hello消息
	err = c.protocol.SendJSON(hello)
	if err != nil {
		logrus.Errorf("发送hello消息失败: %v", err)
		c.protocol.Disconnect()
		c.SetState(StateIdle)
		return err
	}
	logrus.Info("已成功发送hello消息，等待服务器响应")

	// 等待服务器Hello响应
	select {
	case <-c.helloReceived:
		// 成功接收到服务器Hello响应
		logrus.Info("成功接收到服务器hello响应！")
		c.mu.Lock()
		onAudioChannelOpen := c.onAudioChannelOpen
		c.mu.Unlock()

		if onAudioChannelOpen != nil {
			onAudioChannelOpen()
		}
		return nil
	case <-time.After(DefaultHelloTimeout):
		// 超时未收到Hello响应
		logrus.Error("等待服务器hello响应超时")
		c.protocol.Disconnect()
		c.SetState(StateIdle)
		return errors.New("等待服务器Hello响应超时")
	}
}

// CloseAudioChannel 关闭音频通道
func (c *Client) CloseAudioChannel() error {
	// 添加恢复机制，防止任何可能的异常
	defer func() {
		if r := recover(); r != nil {
			logrus.Errorf("关闭音频通道时发生异常: %v", r)
		}
	}()

	c.mu.Lock()
	if c.state == StateIdle {
		c.mu.Unlock()
		return nil
	}
	c.mu.Unlock()

	// 尝试断开连接，如果出现错误，记录但继续处理
	var err error
	func() {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("断开连接时发生异常: %v", r)
				logrus.Error(err)
			}
		}()

		err = c.protocol.Disconnect()
	}()

	// 无论是否出错，都调用断开连接处理程序
	c.handleDisconnected(err)

	// 确保状态设置为空闲
	c.SetState(StateIdle)

	return err
}

// SendStartListening 发送开始监听的消息
func (c *Client) SendStartListening(mode string) error {
	c.mu.Lock()
	if c.state != StateConnecting && c.state != StateIdle && c.state != StateSpeaking {
		c.mu.Unlock()
		return errors.New("客户端状态不允许开始监听")
	}

	// 设置会话ID和监听模式
	if c.sessionID == "" {
		c.sessionID = uuid.New().String()
	}

	// 设置监听模式
	if mode == "" {
		mode = ListenModeManual
	}
	c.listenMode = mode

	sessionID := c.sessionID
	c.mu.Unlock()

	// 发送listen消息
	listen := protocol.ListenMessage{
		SessionID: sessionID,
		Type:      "listen",
		State:     "start",
		Mode:      mode,
	}

	err := c.protocol.SendJSON(listen)
	if err != nil {
		return err
	}

	// 更新状态
	c.SetState(StateListening)
	return nil
}

// SendStopListening 发送停止监听的消息
func (c *Client) SendStopListening() error {
	c.mu.Lock()
	if c.state != StateListening {
		c.mu.Unlock()
		return errors.New("客户端不在监听状态，无法停止监听")
	}

	sessionID := c.sessionID
	c.mu.Unlock()

	// 发送listen消息
	listen := protocol.ListenMessage{
		SessionID: sessionID,
		Type:      "listen",
		State:     "stop",
	}

	return c.protocol.SendJSON(listen)
}

// SendWakeWordDetected 发送唤醒词检测到的消息
func (c *Client) SendWakeWordDetected(text string) error {
	c.mu.Lock()
	if c.state != StateListening && c.state != StateSpeaking && c.state != StateIdle {
		c.mu.Unlock()
		return errors.New("客户端状态不允许发送唤醒词检测")
	}

	// 如果当前不在监听状态，先开始监听
	if c.state != StateListening {
		c.mu.Unlock()
		err := c.SendStartListening(ListenModeAuto)
		if err != nil {
			return err
		}
		c.mu.Lock()
	}

	sessionID := c.sessionID
	c.mu.Unlock()

	// 发送wake word detected消息
	listen := protocol.ListenMessage{
		SessionID: sessionID,
		Type:      "listen",
		State:     "detect",
		Text:      text,
	}

	return c.protocol.SendJSON(listen)
}

// SendAbortSpeaking 发送终止当前会话的消息
func (c *Client) SendAbortSpeaking(reason string) error {
	c.mu.Lock()
	if c.state == StateIdle {
		c.mu.Unlock()
		return nil
	}

	sessionID := c.sessionID
	c.mu.Unlock()

	// 发送abort消息
	abort := protocol.AbortMessage{
		SessionID: sessionID,
		Type:      "abort",
		Reason:    reason,
	}

	return c.protocol.SendJSON(abort)
}

// SendIoTState 发送IoT状态消息
func (c *Client) SendIoTState(states interface{}) error {
	c.mu.Lock()
	if !c.protocol.IsConnected() {
		c.mu.Unlock()
		return errors.New("未连接到服务器")
	}

	sessionID := c.sessionID
	c.mu.Unlock()

	// 发送IoT状态消息
	iotState := protocol.IoTStateMessage{
		SessionID: sessionID,
		Type:      "iot",
		States:    states,
	}

	return c.protocol.SendJSON(iotState)
}

// SendIoTDescriptors 发送IoT描述符消息
func (c *Client) SendIoTDescriptors(descriptors interface{}) error {
	c.mu.Lock()
	if !c.protocol.IsConnected() {
		c.mu.Unlock()
		return errors.New("未连接到服务器")
	}

	sessionID := c.sessionID
	c.mu.Unlock()

	// 发送IoT描述符消息
	iotDesc := protocol.IoTStateMessage{
		SessionID:   sessionID,
		Type:        "iot",
		Descriptors: descriptors,
	}

	return c.protocol.SendJSON(iotDesc)
}

// SendAudioData 发送音频数据
func (c *Client) SendAudioData(data []byte) error {
	c.mu.Lock()
	if c.state != StateListening {
		c.mu.Unlock()
		return errors.New("客户端不在监听状态，无法发送音频数据")
	}
	c.mu.Unlock()

	return c.protocol.SendBinary(data)
}

// 内部事件处理方法

// handleConnected 处理连接成功事件
func (c *Client) handleConnected() {
	logrus.Info("WebSocket已连接")
}

// handleDisconnected 处理连接断开事件
func (c *Client) handleDisconnected(err error) {
	// 添加超时保护
	done := make(chan struct{})

	go func() {
		defer close(done)

		c.mu.Lock()
		oldState := c.state
		c.state = StateIdle
		onAudioChannelClosed := c.onAudioChannelClosed
		onNetworkError := c.onNetworkError
		c.sessionID = ""
		c.mu.Unlock()

		// 如果之前不是空闲状态，触发通道关闭回调
		if oldState != StateIdle && onAudioChannelClosed != nil {
			onAudioChannelClosed()
		}

		// 如果是由于错误导致的断开，触发网络错误回调
		if err != nil && onNetworkError != nil {
			onNetworkError(err)
		}
	}()

	// 等待处理完成或超时
	select {
	case <-done:
		// 成功完成
		return
	case <-time.After(2 * time.Second):
		// 处理超时
		logrus.Warn("处理连接断开事件超时")

		// 强制设置状态为空闲
		c.mu.Lock()
		c.state = StateIdle
		c.sessionID = ""
		c.mu.Unlock()
	}
}

// handleJSONMessage 处理JSON消息
func (c *Client) handleJSONMessage(data []byte) {
	// 记录收到的JSON消息，但不记录太大的数据
	if len(data) < 1000 {
		logrus.Debugf("收到WebSocket JSON消息: %s", string(data))
	} else {
		logrus.Debugf("收到WebSocket JSON消息，长度: %d字节", len(data))
	}

	// 解析消息类型
	var message struct {
		Type string `json:"type"`
	}

	if err := json.Unmarshal(data, &message); err != nil {
		logrus.Errorf("解析WebSocket消息失败: %v", err)
		return
	}

	// 根据消息类型分别处理
	switch message.Type {
	case "hello":
		logrus.Info("识别到服务器hello消息，进行处理")
		c.handleHelloMessage(data)
	case "stt":
		c.handleSTTMessage(data)
	case "tts":
		c.handleTTSMessage(data)
	case "llm":
		c.handleLLMMessage(data)
	case "iot":
		c.handleIoTMessage(data)
	case "error":
		c.handleErrorMessage(data)
	default:
		logrus.Warnf("收到未知类型的WebSocket消息: %s", message.Type)
	}
}

// handleBinaryMessage 处理接收到的二进制消息
func (c *Client) handleBinaryMessage(data []byte) {
	c.mu.Lock()
	// 如果是在监听状态，忽略收到的音频数据
	if c.state == StateListening {
		c.mu.Unlock()
		return
	}

	onAudioData := c.onAudioData
	c.mu.Unlock()

	// 调用音频数据回调
	if onAudioData != nil {
		onAudioData(data)
	}
}

// handleHelloMessage 处理Hello消息
func (c *Client) handleHelloMessage(data []byte) {
	var hello protocol.ServerHelloMessage
	if err := json.Unmarshal(data, &hello); err != nil {
		logrus.Errorf("解析Hello消息失败: %v", err)
		return
	}

	// 验证消息格式
	if hello.Type != "hello" || hello.Transport != "websocket" {
		logrus.Errorf("服务器返回的Hello消息格式不正确")
		c.protocol.Disconnect()
		return
	}

	// 通知等待的goroutine已收到Hello消息
	select {
	case c.helloReceived <- struct{}{}:
	default:
		// 通道已关闭或已经有值了，不需要发送
	}
}

// handleSTTMessage 处理STT消息
func (c *Client) handleSTTMessage(data []byte) {
	var stt protocol.STTMessage
	if err := json.Unmarshal(data, &stt); err != nil {
		logrus.Errorf("解析STT消息失败: %v", err)
		return
	}

	c.mu.Lock()
	onRecognizedText := c.onRecognizedText
	c.mu.Unlock()

	// 调用识别文本回调
	if onRecognizedText != nil {
		onRecognizedText(stt.Text)
	}
}

// handleTTSMessage 处理TTS消息
func (c *Client) handleTTSMessage(data []byte) {
	var tts protocol.TTSMessage
	if err := json.Unmarshal(data, &tts); err != nil {
		logrus.Errorf("解析TTS消息失败: %v", err)
		return
	}

	switch tts.State {
	case "start":
		// TTS开始，切换到播放状态
		c.SetState(StateSpeaking)
	case "stop":
		// TTS结束，切换到空闲状态
		c.SetState(StateIdle)
	case "sentence_start":
		// 句子开始，调用文本回调
		c.mu.Lock()
		onSpeakText := c.onSpeakText
		c.mu.Unlock()

		if onSpeakText != nil && tts.Text != "" {
			onSpeakText(tts.Text)
		}
	}
}

// handleLLMMessage 处理LLM消息
func (c *Client) handleLLMMessage(data []byte) {
	var llm protocol.LLMMessage
	if err := json.Unmarshal(data, &llm); err != nil {
		logrus.Errorf("解析LLM消息失败: %v", err)
		return
	}

	c.mu.Lock()
	onEmotionChanged := c.onEmotionChanged
	c.mu.Unlock()

	// 调用情感变更回调
	if onEmotionChanged != nil {
		onEmotionChanged(llm.Emotion, llm.Text)
	}
}

// handleIoTMessage 处理IoT消息
func (c *Client) handleIoTMessage(data []byte) {
	var msg map[string]interface{}
	if err := json.Unmarshal(data, &msg); err != nil {
		logrus.Errorf("解析IoT消息失败: %v", err)
		return
	}

	// 检查是否包含commands字段
	if commands, ok := msg["commands"].([]interface{}); ok {
		c.mu.Lock()
		onIoTCommand := c.onIoTCommand
		c.mu.Unlock()

		// 调用IoT命令回调
		if onIoTCommand != nil {
			onIoTCommand(commands)
		}
	}
}

// handleErrorMessage 处理错误消息
func (c *Client) handleErrorMessage(data []byte) {
	var errMsg struct {
		Type  string `json:"type"`
		Code  int    `json:"code"`
		Error string `json:"error"`
	}

	if err := json.Unmarshal(data, &errMsg); err != nil {
		logrus.Errorf("解析错误消息失败: %v", err)
		return
	}

	logrus.Errorf("收到服务器错误: 代码=%d, 消息=%s", errMsg.Code, errMsg.Error)

	// 调用网络错误回调
	c.mu.Lock()
	onNetworkError := c.onNetworkError
	c.mu.Unlock()

	if onNetworkError != nil {
		onNetworkError(fmt.Errorf("服务器错误: %s (代码: %d)", errMsg.Error, errMsg.Code))
	}
}

// GetProtocol 获取协议实例
func (c *Client) GetProtocol() protocol.Protocol {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.protocol
}
