package config

import "time"

// 应用程序常量定义
const (
	// 状态常量
	StateIdle      = "idle"
	StateListening = "listening"
	StateSpeaking  = "speaking"

	// 默认配置值
	DefaultServerURL      = "wss://api.tenclass.net/xiaozhi/v1/"
	DefaultDeviceID       = ""
	DefaultToken          = "test-token"
	DefaultBoardType      = "generic"
	DefaultAppVersion     = "1.0.0"
	DefaultLogLevel       = "info"
	DefaultSilenceTimeout = 800 // 毫秒
	DefaultAutoSilenceThreshold = 3 // 秒

	// 超时配置
	HandshakeTimeout = 15 * time.Second
	HeartbeatInterval = 30 * time.Second
	StateChangeTimeout = 3 * time.Second
	CommandTimeout = 3 * time.Second
	MaxRecordingDuration = 10 * time.Second
	AutoInteractionDelay = 500 * time.Millisecond
	CleanupTimeout = 500 * time.Millisecond
	ForcedExitTimeout = 1 * time.Second
	AudioChannelCloseDelay = 50 * time.Millisecond
	StopAudioDelay = 500 * time.Millisecond

	// 缓冲区大小
	AudioChannelBuffer = 100
	VADCheckInterval = 500 * time.Millisecond
	StateCheckInterval = 100 * time.Millisecond
	AutoInteractionWaitInterval = 100 * time.Millisecond

	// VAD配置
	DefaultVADThreshold = 0.5
	DefaultVADMinSilenceDuration = 0.5
	DefaultVADMinSpeechDuration = 0.25
	DefaultVADMaxSpeechDuration = 10.0
	DefaultVADWindowSize = 512
	DefaultVADSampleRate = 16000
)