package interaction

import "errors"

// 交互模块错误定义
var (
	// ErrNotInitialized 未初始化
	ErrNotInitialized = errors.New("interaction manager not initialized")

	// ErrAlreadyRunning 已在运行
	ErrAlreadyRunning = errors.New("interaction manager already running")

	// ErrNotRunning 未运行
	ErrNotRunning = errors.New("interaction manager not running")

	// ErrWakeWordDetectorNil 唤醒词检测器为空
	ErrWakeWordDetectorNil = errors.New("wake word detector is nil")

	// ErrInputManagerNil 输入管理器为空
	ErrInputManagerNil = errors.New("input manager is nil")

	// ErrChannelClosed 通道已关闭
	ErrChannelClosed = errors.New("channel is closed")

	// ErrChannelFull 通道已满
	ErrChannelFull = errors.New("channel is full")

	// ErrTerminalModeFailed 终端模式设置失败
	ErrTerminalModeFailed = errors.New("failed to set terminal mode")

	// ErrInputReadFailed 输入读取失败
	ErrInputReadFailed = errors.New("failed to read input")

	// ErrVADNotEnabled VAD未启用
	ErrVADNotEnabled = errors.New("VAD functionality not enabled")

	// ErrSilenceTimeoutInvalid 静音超时设置无效
	ErrSilenceTimeoutInvalid = errors.New("invalid silence timeout value")

	// ErrAutoInteractionDisabled 自动交互已禁用
	ErrAutoInteractionDisabled = errors.New("auto interaction is disabled")

	// ErrClientNotConnected 客户端未连接
	ErrClientNotConnected = errors.New("client is not connected")
)