package audio

import "errors"

// 音频模块错误定义
var (
	// ErrNotInitialized 音频管理器未初始化
	ErrNotInitialized = errors.New("audio manager not initialized")

	// ErrManagerNil 音频管理器为空
	ErrManagerNil = errors.New("audio manager is nil")

	// ErrRecordingInProgress 录音正在进行中
	ErrRecordingInProgress = errors.New("recording already in progress")

	// ErrNotRecording 当前没有在录音
	ErrNotRecording = errors.New("not currently recording")

	// ErrPlayerNotInitialized 播放器未初始化
	ErrPlayerNotInitialized = errors.New("audio player not initialized")

	// ErrVADNotEnabled VAD功能未启用
	ErrVADNotEnabled = errors.New("VAD functionality not enabled")

	// ErrInvalidConfig 配置无效
	ErrInvalidConfig = errors.New("invalid audio configuration")

	// ErrDeviceNotFound 音频设备未找到
	ErrDeviceNotFound = errors.New("audio device not found")

	// ErrDeviceBusy 音频设备忙碌
	ErrDeviceBusy = errors.New("audio device busy")

	// ErrPermissionDenied 权限被拒绝
	ErrPermissionDenied = errors.New("permission denied to access audio device")
)