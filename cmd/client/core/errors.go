package core

import "errors"

// 核心模块错误定义
var (
	// ErrDeviceNotActivated 设备未激活
	ErrDeviceNotActivated = errors.New("device not activated")

	// ErrInitializationFailed 初始化失败
	ErrInitializationFailed = errors.New("application initialization failed")

	// ErrConfigurationInvalid 配置无效
	ErrConfigurationInvalid = errors.New("invalid configuration")

	// ErrNotInitialized 未初始化
	ErrNotInitialized = errors.New("application not initialized")

	// ErrAlreadyRunning 已在运行
	ErrAlreadyRunning = errors.New("application already running")

	// ErrNotRunning 未运行
	ErrNotRunning = errors.New("application not running")

	// ErrClientNil 客户端为空
	ErrClientNil = errors.New("client instance is nil")

	// ErrFlagsNil 命令行参数为空
	ErrFlagsNil = errors.New("flags configuration is nil")

	// ErrConnectionFailed 连接失败
	ErrConnectionFailed = errors.New("failed to connect to server")

	// ErrSetupFailed 设置失败
	ErrSetupFailed = errors.New("failed to setup application components")

	// ErrWakeWordNotInitialized 唤醒词检测器未初始化
	ErrWakeWordNotInitialized = errors.New("wake word detector not initialized")
)