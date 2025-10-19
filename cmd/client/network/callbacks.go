package network

import (
	"encoding/json"
	"time"

	"github.com/justa-cai/xiaozhi-go/internal/client"
	"github.com/sirupsen/logrus"
)

// CallbackHandler 网络回调处理器
type CallbackHandler struct {
	clientInstance *client.Client
	audioHandler   func([]byte)     // 音频数据处理回调
	ttsStartHandler func()          // TTS开始回调
	ttsStopHandler  func()          // TTS停止回调
	helloHandler    func(map[string]interface{}) // Hello消息处理回调
	sttHandler      func(string)    // STT识别结果回调
}

// NewCallbackHandler 创建新的回调处理器
func NewCallbackHandler(clientInstance *client.Client) *CallbackHandler {
	return &CallbackHandler{
		clientInstance: clientInstance,
	}
}

// SetAudioHandler 设置音频数据处理器
func (ch *CallbackHandler) SetAudioHandler(handler func([]byte)) {
	ch.audioHandler = handler
}

// SetTTSStartHandler 设置TTS开始处理器
func (ch *CallbackHandler) SetTTSStartHandler(handler func()) {
	ch.ttsStartHandler = handler
}

// SetTTSStopHandler 设置TTS停止处理器
func (ch *CallbackHandler) SetTTSStopHandler(handler func()) {
	ch.ttsStopHandler = handler
}

// SetHelloHandler 设置Hello消息处理器
func (ch *CallbackHandler) SetHelloHandler(handler func(map[string]interface{})) {
	ch.helloHandler = handler
}

// SetSTTHandler 设置STT消息处理器
func (ch *CallbackHandler) SetSTTHandler(handler func(string)) {
	ch.sttHandler = handler
}

// HandleJSONMessage 处理JSON消息
func (ch *CallbackHandler) HandleJSONMessage(data []byte, verboseLogging bool) {
	// 尝试解析JSON格式
	var jsonData interface{}
	if err := json.Unmarshal(data, &jsonData); err == nil {
		if typeMap, ok := jsonData.(map[string]interface{}); ok {
			if msgType, exists := typeMap["type"]; exists {
				switch msgType {
				case "hello":
					ch.handleHelloMessage(typeMap)
				case "tts":
					ch.handleTTSMessage(typeMap)
				case "stt":
					ch.handleSTTMessage(typeMap)
				default:
					logrus.Debugf("收到未处理的消息类型: %v", msgType)
				}
			}
		}
	}
}

// handleHelloMessage 处理Hello消息
func (ch *CallbackHandler) handleHelloMessage(typeMap map[string]interface{}) {
	logrus.Info("收到服务器hello消息")

	// 检查是否包含音频参数
	if audioParams, ok := typeMap["audio_params"].(map[string]interface{}); ok {
		logrus.Info("收到服务器hello消息，包含音频参数")

		// 提取音频参数
		sampleRate, _ := audioParams["sample_rate"].(float64)
		channels, _ := audioParams["channels"].(float64)
		frameDuration, _ := audioParams["frame_duration"].(float64)
		format, _ := audioParams["format"].(string)

		// 验证音频参数有效性
		if sampleRate > 0 && channels > 0 && frameDuration > 0 && format != "" {
			logrus.Infof("重新初始化解码器: format=%s, sample_rate=%v, channels=%v, frame_duration=%v",
				format, sampleRate, channels, frameDuration)

			// 调用hello处理回调
			if ch.helloHandler != nil {
				audioParamsMap := map[string]interface{}{
					"format":         format,
					"sample_rate":    int(sampleRate),
					"channels":       int(channels),
					"frame_duration": int(frameDuration),
				}
				ch.helloHandler(audioParamsMap)
			}
		}
	}
}

// handleTTSMessage 处理TTS消息
func (ch *CallbackHandler) handleTTSMessage(typeMap map[string]interface{}) {
	if state, exists := typeMap["state"].(string); exists {
		if state == "start" || state == "sentence_start" {
			logrus.Info("🎵 检测到TTS播放开始，暂停VAD检测...")

			// 调用TTS开始回调
			if ch.ttsStartHandler != nil {
				ch.ttsStartHandler()
			}
		} else if state == "stop" {
			logrus.Info("🔄 检测到TTS播放结束，更新客户端状态...")

			// 手动更新客户端状态从speaking到idle
			if ch.clientInstance != nil {
				currentState := ch.clientInstance.GetState()
				logrus.Infof("📝 TTS停止前客户端状态: %s", currentState)
				if currentState == client.StateSpeaking {
					ch.clientInstance.SetState(client.StateIdle)
					logrus.Infof("✅ 已将客户端状态从 %s 更新为 %s", client.StateSpeaking, client.StateIdle)
				}
			}

			// 调用TTS停止回调
			if ch.ttsStopHandler != nil {
				ch.ttsStopHandler()
			}
		}
	}
}

// handleSTTMessage 处理STT消息
func (ch *CallbackHandler) handleSTTMessage(typeMap map[string]interface{}) {
	if text, exists := typeMap["text"].(string); exists {
		logrus.Infof("🎯 收到语音识别结果: %s", text)

		// 调用STT回调
		if ch.sttHandler != nil {
			ch.sttHandler(text)
		}
	}
}

// HandleBinaryMessage 处理二进制消息
func (ch *CallbackHandler) HandleBinaryMessage(data []byte, verboseLogging bool) {
	// 处理Opus编码的音频数据
	if ch.audioHandler != nil {
		ch.audioHandler(data)
	}
}

// HandlePingMessage 处理Ping消息
func (ch *CallbackHandler) HandlePingMessage() {
	// 默认的Ping消息处理逻辑
	logrus.Debugf("心跳包已发送: %d", time.Now().Unix())
}

// SetupClientCallbacks 设置客户端回调
func (ch *CallbackHandler) SetupClientCallbacks() {
	if ch.clientInstance == nil {
		return
	}

	// 状态变更回调
	ch.clientInstance.SetOnStateChanged(func(oldState, newState string) {
		logrus.Infof("客户端状态变更: %s -> %s", oldState, newState)
	})

	// 网络错误回调
	ch.clientInstance.SetOnNetworkError(func(err error) {
		logrus.Errorf("网络错误: %v", err)
	})

	// 识别文本回调
	ch.clientInstance.SetOnRecognizedText(func(text string) {
		logrus.Infof("识别到文本: %s", text)
	})

	// 朗读文本回调
	ch.clientInstance.SetOnSpeakText(func(text string) {
		logrus.Infof("AI回复: %s", text)
	})

	// 情感变更回调
	ch.clientInstance.SetOnEmotionChanged(func(emotion, text string) {
		logrus.Infof("情感变更: %s, 表情: %s", emotion, text)
	})

	// IoT命令回调
	ch.clientInstance.SetOnIoTCommand(func(commands []interface{}) {
		logrus.Infof("收到IoT命令: %v", commands)
	})

	// 音频通道打开回调
	ch.clientInstance.SetOnAudioChannelOpen(func() {
		logrus.Info("音频通道已打开")
	})

	// 音频通道关闭回调
	ch.clientInstance.SetOnAudioChannelClosed(func() {
		logrus.Info("音频通道已关闭")
	})
}