package device

import "errors"

// 设备模块错误定义
var (
	// ErrDeviceInfoNil 设备信息为空
	ErrDeviceInfoNil = errors.New("device info is nil")

	// ErrDeviceIDEmpty 设备ID为空
	ErrDeviceIDEmpty = errors.New("device ID is empty")

	// ErrClientIDEmpty 客户端ID为空
	ErrClientIDEmpty = errors.New("client ID is empty")

	// ErrMACAddressNotFound MAC地址未找到
	ErrMACAddressNotFound = errors.New("MAC address not found")

	// ErrInvalidBoardType 无效的板型
	ErrInvalidBoardType = errors.New("invalid board type")

	// ErrInvalidVersion 无效的版本号
	ErrInvalidVersion = errors.New("invalid version number")

	// ErrActivationFailed 激活失败
	ErrActivationFailed = errors.New("device activation failed")

	// ErrNotActivated 设备未激活
	ErrNotActivated = errors.New("device not activated")

	// ErrAlreadyActivated 设备已激活
	ErrAlreadyActivated = errors.New("device already activated")

	// ErrActivationCodeEmpty 激活码为空
	ErrActivationCodeEmpty = errors.New("activation code is empty")

	// ErrFirmwareVersionEmpty 固件版本为空
	ErrFirmwareVersionEmpty = errors.New("firmware version is empty")

	// ErrMQTPEndpointEmpty MQTT端点为空
	ErrMQTPEndpointEmpty = errors.New("MQTT endpoint is empty")

	// ErrMQTTClientIDEmpty MQTT客户端ID为空
	ErrMQTTClientIDEmpty = errors.New("MQTT client ID is empty")

	// ErrNetworkInterfaceNotFound 网络接口未找到
	ErrNetworkInterfaceNotFound = errors.New("network interface not found")

	// ErrPermissionDenied 权限被拒绝
	ErrPermissionDenied = errors.New("permission denied to access network interface")
)