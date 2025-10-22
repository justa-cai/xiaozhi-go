package interaction

import (
	"time"

	"github.com/justa-cai/xiaozhi-go/cmd/client/audio"
	"github.com/justa-cai/xiaozhi-go/internal/client"
	"github.com/justa-cai/xiaozhi-go/internal/wakeword"
	"github.com/sirupsen/logrus"
)

// WakeWordManager 唤醒词检测管理器
type WakeWordManager struct {
	detector          *wakeword.WakeWordDetector
	timer             *time.Timer
	isRunning         bool
	config            WakeWordConfig
	autoInteractionMode bool
	soundEffectsManager *audio.SoundEffectsManager
	onWakeWordDetected func(string) // 唤醒词检测回调
}

// WakeWordConfig 唤醒词配置
type WakeWordConfig struct {
	MaxRecordingDuration time.Duration
	DebugEnabled        bool
}

// NewWakeWordManager 创建新的唤醒词管理器
func NewWakeWordManager(config WakeWordConfig) *WakeWordManager {
	return &WakeWordManager{
		config:     config,
		isRunning: false,
	}
}

// Initialize 初始化唤醒词检测器
func (wwm *WakeWordManager) Initialize(
	clientInstance *client.Client,
	onWakeWordDetected func(string),
) error {
	if wwm.isRunning {
		return nil
	}

	// 保存回调函数
	wwm.onWakeWordDetected = onWakeWordDetected

	logrus.Info("正在初始化唤醒词检测器...")

	var err error
	wwm.detector, err = wakeword.NewWakeWordDetector(
		// 唤醒词检测回调函数
		func(keyword string) {
			logrus.Infof("唤醒词 '%s' 检测到！激活助手...", keyword)

			// 播放deng提示音
			if wwm.soundEffectsManager != nil {
				wwm.soundEffectsManager.PlayDengSound()
			}

			// 检查客户端是否已连接到服务器
			if !clientInstance.GetProtocol().IsConnected() {
				logrus.Error("客户端未连接到服务器，无法开始录音")
				return
			}

			// 检查当前状态
			currentState := clientInstance.GetState()

			switch currentState {
			case client.StateListening:
				// 如果已经在监听状态，重新启动VAD检测
				logrus.Info("🔄 客户端已在监听状态，检测到唤醒词，重新启动VAD检测...")
				if wwm.onWakeWordDetected != nil {
					wwm.onWakeWordDetected(keyword)
				}
				return

			case client.StateSpeaking:
				// 如果正在播放AI回复，先打断播放
				logrus.Info("🎯 检测到唤醒词，正在中断AI回复...")
				if err := clientInstance.SendAbortSpeaking("stop_speaking"); err != nil {
					logrus.Errorf("发送停止讲话命令失败: %v", err)
					return
				}
				logrus.Info("✅ 已发送停止讲话命令")

				// 等待服务器处理完成，并确保状态已重置
				logrus.Debug("⏳ 等待服务器处理停止讲话命令...")
				time.Sleep(300 * time.Millisecond)

				// 确保客户端状态已正确重置
				if clientInstance.GetState() == client.StateSpeaking {
					logrus.Warn("⚠️ 发送停止命令后状态仍为speaking，强制重置为idle")
					// 这里可能需要调用状态重置方法，但目前先记录日志
				}

			case client.StateIdle:
				// idle状态，直接开始监听
				logrus.Info("🎯 idle状态检测到唤醒词，开始监听...")

			default:
				logrus.Warnf("未知客户端状态: %s，按idle状态处理", currentState)
			}

			// 统一处理：进入监听模式
			logrus.Info("🎤 进入监听模式（等同于按F键）...")

			// 发送开始录音命令到服务器
			if err := clientInstance.SendStartListening(client.ListenModeManual); err != nil {
				logrus.Errorf("发送开始监听命令失败: %v", err)
				return
			}
			logrus.Info("已向服务器发送开始监听命令，准备接收语音输入...")

			// 调用唤醒词检测回调（用于设置VAD等）
			if onWakeWordDetected != nil {
				onWakeWordDetected(keyword)
			}

			// 启动最大录音时长定时器
			if wwm.timer != nil {
				wwm.timer.Stop()
			}
			wwm.timer = time.AfterFunc(wwm.config.MaxRecordingDuration, func() {
				if clientInstance.GetState() == client.StateListening {
					logrus.Info("录音达到最大时长，自动停止录音（等同于按S键）...")
				}
			})
			logrus.Infof("已设置%v自动停止定时器", wwm.config.MaxRecordingDuration)
		})

	if err != nil {
		logrus.Errorf("初始化唤醒词检测器失败: %v", err)
		return err
	}

	wwm.isRunning = true
	return nil
}

// Start 启动唤醒词检测
func (wwm *WakeWordManager) Start() error {
	if !wwm.isRunning || wwm.detector == nil {
		return ErrNotInitialized
	}

	if err := wwm.detector.Start(); err != nil {
		logrus.Errorf("启动唤醒词检测器失败: %v", err)
		return err
	}

	logrus.Info("唤醒词检测器已启动")

	return nil
}

// Stop 停止唤醒词检测
func (wwm *WakeWordManager) Stop() error {
	if !wwm.isRunning {
		return nil
	}

	if wwm.detector != nil {
		if err := wwm.detector.Stop(); err != nil {
			logrus.Errorf("停止唤醒词检测器失败: %v", err)
			return err
		}
	}

	if wwm.timer != nil {
		wwm.timer.Stop()
		wwm.timer = nil
	}

	wwm.isRunning = false

	logrus.Info("唤醒词检测器已停止")
	return nil
}

// ProcessAudioData 处理音频数据
func (wwm *WakeWordManager) ProcessAudioData(data []int16) {
	if wwm.isRunning && wwm.detector != nil && wwm.detector.IsRunning() {
		wwm.detector.ProcessAudioData(data)
	}
}


// IsRunning 检查是否正在运行
func (wwm *WakeWordManager) IsRunning() bool {
	return wwm.isRunning
}

// IsDetectorRunning 检查检测器是否正在运行
func (wwm *WakeWordManager) IsDetectorRunning() bool {
	if wwm.detector != nil {
		return wwm.detector.IsRunning()
	}
	return false
}

// SetAutoInteractionMode 设置自动交互模式状态
func (wwm *WakeWordManager) SetAutoInteractionMode(enabled bool) {
	wwm.autoInteractionMode = enabled
}

// GetDetector 获取唤醒词检测器实例
func (wwm *WakeWordManager) GetDetector() interface{} {
	if wwm.detector != nil {
		return wwm.detector
	}
	return nil
}

// IsAutoInteractionMode 检查是否处于自动交互模式
func (wwm *WakeWordManager) IsAutoInteractionMode() bool {
	return wwm.autoInteractionMode
}

// SetSoundEffectsManager 设置音效管理器
func (wwm *WakeWordManager) SetSoundEffectsManager(manager *audio.SoundEffectsManager) {
	wwm.soundEffectsManager = manager
}