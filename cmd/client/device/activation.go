package device

import (
	"github.com/justa-cai/xiaozhi-go/internal/ota"
	"github.com/sirupsen/logrus"
)

// ActivationManager 设备激活管理器
type ActivationManager struct {
	deviceInfo *DeviceInfo
}

// NewActivationManager 创建新的激活管理器
func NewActivationManager(deviceInfo *DeviceInfo) *ActivationManager {
	return &ActivationManager{
		deviceInfo: deviceInfo,
	}
}

// RunActivation 运行激活流程
func (am *ActivationManager) RunActivation() error {
	if am.deviceInfo == nil {
		return ErrDeviceInfoNil
	}

	logrus.Info("开始执行设备激活流程...")

	// 创建OTA客户端
	otaClient := ota.NewOTAClient(
		am.deviceInfo.GetID(),
		am.deviceInfo.GetAppVersion(),
		am.deviceInfo.GetBoardType(),
	)

	// 请求激活
	resp, err := otaClient.RequestActivation()
	if err != nil {
		logrus.Fatalf("设备激活失败: %v", err)
		return err
	}

	logrus.Info("设备激活成功!")
	logrus.Infof("激活码: %s", resp.Activation.Code)
	logrus.Infof("固件版本: %s", resp.Firmware.Version)
	logrus.Infof("MQTT配置: 端点=%s, 客户端ID=%s",
		resp.MQTT.Endpoint, resp.MQTT.ClientID)

	// 更新设备激活状态
	am.deviceInfo.SetActivated(true)

	return nil
}

// CheckActivationStatus 检查设备激活状态
func (am *ActivationManager) CheckActivationStatus() (bool, error) {
	if am.deviceInfo == nil {
		return false, ErrDeviceInfoNil
	}

	// 创建OTA客户端
	otaClient := ota.NewOTAClient(
		am.deviceInfo.GetID(),
		am.deviceInfo.GetAppVersion(),
		am.deviceInfo.GetBoardType(),
	)

	// 检查激活状态
	activated, err := otaClient.CheckActivationStatus()
	if err != nil {
		logrus.Errorf("检查设备激活状态失败: %v", err)
		return false, err
	}

	// 更新设备激活状态
	am.deviceInfo.SetActivated(activated)

	return activated, nil
}

// IsActivated 检查设备是否已激活
func (am *ActivationManager) IsActivated() bool {
	if am.deviceInfo == nil {
		return false
	}
	return am.deviceInfo.IsDeviceActivated()
}

// GetDeviceInfo 获取设备信息
func (am *ActivationManager) GetDeviceInfo() *DeviceInfo {
	return am.deviceInfo
}

// ActivationResponse 激活响应（如果需要自定义处理）
type ActivationResponse struct {
	Code        string
	Firmware    FirmwareInfo
	MQTT        MQTTConfig
}

// FirmwareInfo 固件信息
type FirmwareInfo struct {
	Version string
	URL     string
	Size    int64
	MD5     string
}

// MQTTConfig MQTT配置
type MQTTConfig struct {
	Endpoint string
	ClientID string
	Username string
	Password string
}

// 验证激活响应
func (ar *ActivationResponse) Validate() error {
	if ar.Code == "" {
		return ErrActivationCodeEmpty
	}
	if ar.Firmware.Version == "" {
		return ErrFirmwareVersionEmpty
	}
	if ar.MQTT.Endpoint == "" {
		return ErrMQTPEndpointEmpty
	}
	if ar.MQTT.ClientID == "" {
		return ErrMQTTClientIDEmpty
	}
	return nil
}