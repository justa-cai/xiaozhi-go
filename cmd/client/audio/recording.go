package audio

import (
	"runtime"
	"time"

	"github.com/justa-cai/xiaozhi-go/internal/client"
	"github.com/sirupsen/logrus"
)

// RecordingController 录音控制器
type RecordingController struct {
	manager      *Manager
	audioChan    chan []byte
	sendToServer bool
	isRecording  bool
}

// NewRecordingController 创建新的录音控制器
func NewRecordingController(manager *Manager) *RecordingController {
	return &RecordingController{
		manager:      manager,
		audioChan:    nil,
		sendToServer: false,
		isRecording:  false,
	}
}

// StartRecording 开始录音并发送数据到服务器
func (rc *RecordingController) StartRecording(clientInstance *client.Client, enableWakeWord bool) error {
	if rc.manager == nil || !rc.manager.IsInitialized() {
		return ErrNotInitialized
	}

	// 添加调用来源信息，以便调试
	pc, file, line, ok := runtime.Caller(1)
	callerInfo := "unknown"
	if ok {
		funcName := runtime.FuncForPC(pc).Name()
		fileName := file[len(file)-20:] // 只取文件名后20个字符
		callerInfo = fileName + ":" + string(rune(line)) + " (" + funcName + ")"
	}
	logrus.Infof("🎤 StartRecording 调用来源: %s", callerInfo)

	currentState := "unknown"
	if clientInstance != nil {
		currentState = clientInstance.GetState()
	}
	logrus.Infof("🎤 StartRecording: 客户端当前状态: %s, enableWakeWord: %v", currentState, enableWakeWord)

	// 如果客户端不在监听状态，确保先发送开始监听命令
	if clientInstance != nil && clientInstance.GetState() != client.StateListening {
		logrus.Info("🎤 StartRecording: 客户端不在监听状态，发送开始监听命令...")
		if err := clientInstance.SendStartListening(client.ListenModeManual); err != nil {
			logrus.Errorf("发送开始监听命令失败: %v", err)
			return err
		}
		logrus.Info("已向服务器发送开始监听命令")
	}

	// 检查是否已经在录音状态，避免重复设置
	if rc.audioChan != nil {
		logrus.Info("🎤 StartRecording: 录音通道已存在，激活发送到服务器")
		rc.sendToServer = true
		return nil
	}

	logrus.Info("🎤 StartRecording: 创建新的录音数据通道")
	// 创建一个带缓冲的通道来接收音频数据
	rc.audioChan = make(chan []byte, 100) // 足够大的缓冲区

	// 启动一个单独的goroutine处理音频数据发送
	go rc.handleAudioData(clientInstance)

	// 启用音频数据发送到服务器
	rc.sendToServer = true
	rc.isRecording = true
	logrus.Infof("🎤 StartRecording: 已设置 sendToServer = true，当前状态: %s", clientInstance.GetState())

	// 在非唤醒词模式下启动音频管理器录音
	if !enableWakeWord && !rc.manager.IsRecording() {
		logrus.Info("🎤 StartRecording: 启动音频管理器录音...")
		if err := rc.manager.StartRecording(); err != nil {
			logrus.Errorf("开始录音失败: %v，将无法发送语音", err)
			// 清理资源
			if rc.audioChan != nil {
				close(rc.audioChan)
				rc.audioChan = nil
			}
			rc.sendToServer = false
			rc.isRecording = false
			return err
		} else {
			logrus.Info("音频管理器录音已启动")
		}
	} else {
		logrus.Debugf("🎤 StartRecording: 音频管理器已在录音中 (%v) 或处于唤醒词检测模式 (%v)",
			rc.manager.IsRecording(), enableWakeWord)
	}

	logrus.Info("🎤 StartRecording: 录音数据发送已激活，等待音频输入...")
	return nil
}

// StopRecording 停止录音
func (rc *RecordingController) StopRecording(clientInstance *client.Client, enableWakeWord bool) {
	if rc.manager == nil {
		return
	}

	// 停止向服务器发送音频数据
	if rc.audioChan != nil {
		logrus.Debug("关闭录音数据通道")
		time.Sleep(50 * time.Millisecond)
		close(rc.audioChan)
		rc.audioChan = nil
	} else {
		logrus.Debug("录音数据通道已为nil，无需关闭")
	}

	// 禁用音频数据发送到服务器
	rc.sendToServer = false
	rc.isRecording = false
	logrus.Debug("已设置 sendToServer = false")

	// 在非唤醒词模式下停止音频管理器录音
	if !enableWakeWord {
		if err := rc.manager.StopRecording(); err != nil {
			logrus.Errorf("停止录音失败: %v", err)
		} else {
			logrus.Info("已停止录音")
		}
	} else {
		logrus.Info("唤醒词检测模式：保持音频输入运行以继续检测唤醒词")
	}

	// 向服务器发送停止监听的消息
	if clientInstance != nil {
		currentState := clientInstance.GetState()
		if currentState == client.StateListening {
			if err := clientInstance.SendStopListening(); err != nil {
				logrus.Errorf("发送停止监听消息失败: %v", err)
			} else {
				logrus.Info("已向服务器发送停止监听消息")
			}
		}
	}
}

// handleAudioData 处理音频数据发送
func (rc *RecordingController) handleAudioData(clientInstance *client.Client) {
	if clientInstance == nil {
		return
	}

	logrus.Info("🎤 音频数据发送goroutine已启动")
	packetCount := 0
	for data := range rc.audioChan {
		packetCount++
		// logrus.Debugf("从通道接收到音频数据，准备发送到服务器，大小: %d 字节 (包 #%d)", len(data), packetCount)

		// 发送音频数据到服务器
		startTime := time.Now()
		err := clientInstance.SendAudioData(data)
		elapsed := time.Since(startTime)

		if err != nil {
			logrus.Errorf("发送音频数据失败: %v", err)
		} else {
			// logrus.Debugf("音频数据已成功发送到服务器，大小: %d 字节，耗时: %v (包 #%d)", len(data), elapsed, packetCount)
			if elapsed > 100*time.Millisecond {
				logrus.Warnf("发送音频数据耗时较长: %v，数据大小: %d字节 (包 #%d)", elapsed, len(data), packetCount)
			}
		}
	}
	logrus.Infof("🎤 音频数据处理已停止，总共处理了 %d 个数据包", packetCount)
}

// IsRecording 检查是否正在录音
func (rc *RecordingController) IsRecording() bool {
	return rc.isRecording
}

// GetAudioChannel 获取音频通道
func (rc *RecordingController) GetAudioChannel() chan []byte {
	return rc.audioChan
}

// ShouldSendToServer 检查是否应该发送到服务器
func (rc *RecordingController) ShouldSendToServer() bool {
	return rc.sendToServer
}

// SetupAudioCallback 设置音频数据回调
func (rc *RecordingController) SetupAudioCallback(name string, enableWakeWord bool) {
	if rc.manager == nil {
		return
	}

	rc.manager.AddAudioDataCallback(name, func(data []byte) {
		if rc.sendToServer && rc.audioChan != nil {
			// 发送到通道，不阻塞
			select {
			case rc.audioChan <- data:
				// logrus.Debugf("音频数据已发送到通道，大小: %d 字节 (%s)", len(data), name)
			default:
				// 通道已满，丢弃此数据包
				logrus.Warn("音频数据通道已满，丢弃数据包 (%s)", name)
			}
		}
	})
}

// SetupPCMCallback 设置PCM数据回调
func (rc *RecordingController) SetupPCMCallback(name string, callback func([]int16)) {
	if rc.manager == nil {
		return
	}

	rc.manager.AddPCMDataCallback(name, func(data []int16, size int) {
		// 复制数据以避免竞争条件
		dataCopy := make([]int16, size)
		copy(dataCopy, data[:size])

		if callback != nil {
			callback(dataCopy)
		}
	})
}
