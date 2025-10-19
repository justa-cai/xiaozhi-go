package device

import (
	"crypto/md5"
	"crypto/rand"
	"fmt"
	"net"
	"time"

	"github.com/sirupsen/logrus"
)

// DeviceInfo 设备信息
type DeviceInfo struct {
	ID          string
	MACAddress  string
	ClientID    string
	BoardType   string
	AppVersion  string
	IsActivated bool
}

// NewDeviceInfo 创建新的设备信息
func NewDeviceInfo(deviceID, boardType, appVersion string) *DeviceInfo {
	// 如果没有提供设备ID，则获取MAC地址
	if deviceID == "" {
		macAddr, err := getMACAddress()
		if err != nil {
			logrus.Warnf("无法获取MAC地址: %v", err)
			deviceID = fmt.Sprintf("device-%d", time.Now().Unix())
			logrus.Infof("生成临时设备ID: %s", deviceID)
		} else {
			deviceID = macAddr
		}
	}

	return &DeviceInfo{
		ID:          deviceID,
		MACAddress:  deviceID,
		ClientID:    generateUUID(deviceID),
		BoardType:   boardType,
		AppVersion:  appVersion,
		IsActivated: false,
	}
}

// getMACAddress 获取本机MAC地址
func getMACAddress() (string, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return "", err
	}

	for _, i := range interfaces {
		if i.Flags&net.FlagUp != 0 && i.Flags&net.FlagLoopback == 0 {
			if len(i.HardwareAddr) > 0 {
				return fmt.Sprintf("%02x:%02x:%02x:%02x:%02x:%02x",
					i.HardwareAddr[0], i.HardwareAddr[1], i.HardwareAddr[2],
					i.HardwareAddr[3], i.HardwareAddr[4], i.HardwareAddr[5]), nil
			}
		}
	}

	return "", fmt.Errorf("未找到有效的网络接口")
}

// generateUUID 基于MAC地址生成UUID
func generateUUID(macAddr string) string {
	// 如果MAC地址为空，使用随机数据
	var data []byte
	if macAddr == "" {
		data = make([]byte, 16)
		rand.Read(data)
	} else {
		// 使用MAC地址作为种子计算MD5
		h := md5.New()
		h.Write([]byte(macAddr))
		data = h.Sum(nil)
	}

	// 设置UUID版本 (版本4)
	data[6] = (data[6] & 0x0F) | 0x40
	// 设置变体
	data[8] = (data[8] & 0x3F) | 0x80

	// 按UUID格式转换为字符串
	return fmt.Sprintf("%x-%x-%x-%x-%x",
		data[0:4], data[4:6], data[6:8], data[8:10], data[10:16])
}

// GetID 获取设备ID
func (di *DeviceInfo) GetID() string {
	return di.ID
}

// GetMACAddress 获取MAC地址
func (di *DeviceInfo) GetMACAddress() string {
	return di.MACAddress
}

// GetClientID 获取客户端ID
func (di *DeviceInfo) GetClientID() string {
	return di.ClientID
}

// GetBoardType 获取板型
func (di *DeviceInfo) GetBoardType() string {
	return di.BoardType
}

// GetAppVersion 获取应用版本
func (di *DeviceInfo) GetAppVersion() string {
	return di.AppVersion
}

// IsDeviceActivated 检查设备是否已激活
func (di *DeviceInfo) IsDeviceActivated() bool {
	return di.IsActivated
}

// SetActivated 设置激活状态
func (di *DeviceInfo) SetActivated(activated bool) {
	di.IsActivated = activated
}

// String 返回设备信息的字符串表示
func (di *DeviceInfo) String() string {
	return fmt.Sprintf("Device{ID: %s, MAC: %s, ClientID: %s, Board: %s, Version: %s, Activated: %v}",
		di.ID, di.MACAddress, di.ClientID, di.BoardType, di.AppVersion, di.IsActivated)
}

// ToMap 转换为map格式
func (di *DeviceInfo) ToMap() map[string]interface{} {
	return map[string]interface{}{
		"device_id":    di.ID,
		"mac_address":  di.MACAddress,
		"client_id":    di.ClientID,
		"board_type":   di.BoardType,
		"app_version":  di.AppVersion,
		"is_activated": di.IsActivated,
	}
}

// LogInfo 记录设备信息到日志
func (di *DeviceInfo) LogInfo() {
	logrus.Infof("使用设备ID: %s", di.ID)
	logrus.Infof("使用客户端ID: %s", di.ClientID)
	logrus.Infof("设备板型: %s", di.BoardType)
	logrus.Infof("应用版本: %s", di.AppVersion)
	logrus.Infof("设备激活状态: %v", di.IsActivated)
}

// Validate 验证设备信息
func (di *DeviceInfo) Validate() error {
	if di.ID == "" {
		return fmt.Errorf("设备ID不能为空")
	}
	if di.ClientID == "" {
		return fmt.Errorf("客户端ID不能为空")
	}
	if di.BoardType == "" {
		return fmt.Errorf("设备板型不能为空")
	}
	if di.AppVersion == "" {
		return fmt.Errorf("应用版本不能为空")
	}
	return nil
}