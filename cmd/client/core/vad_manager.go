package core

import (
	"sync"
	"time"

	"github.com/justa-cai/xiaozhi-go/cmd/client/audio"
	"github.com/justa-cai/xiaozhi-go/internal/client"
	"github.com/sirupsen/logrus"
)

// VADManager 统一的VAD管理器，协调音频管理器VAD和唤醒词检测器VAD
type VADManager struct {
	// 核心组件
	audioManager    *audio.Manager
	wakeWordManager interface{} // *interaction.WakeWordManager
	clientInstance  *client.Client
	recordingController interface{} // *audio.RecordingController

	// 配置
	silenceThreshold time.Duration // 静音阈值（秒）
	enableWakeWord   bool

	// 状态管理
	mu               sync.RWMutex
	isActive         bool
	isPaused         bool
	lastResumeTime   time.Time
	gracePeriod      time.Duration // 宽限期

	// 回调
	onSilenceTimeout func() // 静音超时回调
}

// VADConfig VAD管理器配置
type VADConfig struct {
	SilenceThreshold time.Duration // 静音阈值
	EnableWakeWord   bool          // 是否启用唤醒词
	GracePeriod      time.Duration // VAD恢复后的宽限期
}

// NewVADManager 创建新的VAD管理器
func NewVADManager(
	audioManager *audio.Manager,
	wakeWordManager interface{},
	recordingController interface{},
	config VADConfig,
) *VADManager {
	return &VADManager{
		audioManager:        audioManager,
		wakeWordManager:     wakeWordManager,
		clientInstance:      nil, // 稍后设置
		recordingController: recordingController,
		silenceThreshold:    config.SilenceThreshold,
		enableWakeWord:      config.EnableWakeWord,
		gracePeriod:         config.GracePeriod,
		isActive:           false,
		isPaused:           false,
	}
}

// SetClientInstance 设置客户端实例
func (vm *VADManager) SetClientInstance(clientInstance *client.Client) {
	vm.mu.Lock()
	defer vm.mu.Unlock()
	vm.clientInstance = clientInstance
}

// Start 启动VAD管理器
func (vm *VADManager) Start() error {
	vm.mu.Lock()
	defer vm.mu.Unlock()

	if vm.isActive {
		return nil
	}

	logrus.Info("🎯 启动统一VAD管理器")

	// 重置VAD状态和计时器
	vm.lastResumeTime = time.Now()
	vm.isPaused = false

	// 重置底层VAD检测器状态
	if vm.audioManager != nil && vm.audioManager.VAD() != nil {
		vm.audioManager.VAD().Reset()
		logrus.Debug("🔄 VAD检测器状态已重置")
	}

	// 禁用音频管理器的内部VAD超时机制，完全由我们控制
	if vm.audioManager != nil && vm.audioManager.VAD() != nil {
		// 设置我们的回调函数
		vm.audioManager.SetVADCallbacks(
			nil, // 不需要语音开始回调
			vm.handleSilenceDetection,
		)
		logrus.Info("✅ 音频管理器VAD回调已设置")
	}

	vm.isActive = true
	return nil
}

// Stop 停止VAD管理器
func (vm *VADManager) Stop() {
	vm.mu.Lock()
	defer vm.mu.Unlock()

	if !vm.isActive {
		return
	}

	logrus.Info("🛑 停止统一VAD管理器")
	vm.isActive = false
}

// Pause 暂停VAD检测（用于TTS播放期间）
func (vm *VADManager) Pause() {
	vm.mu.Lock()
	defer vm.mu.Unlock()

	if !vm.isActive || vm.isPaused {
		return
	}

	vm.isPaused = true
	logrus.Debug("🔇 VAD管理器已暂停（TTS播放期间）")

	// 暂停音频管理器VAD
	if vm.audioManager != nil && vm.audioManager.VAD() != nil {
		vm.audioManager.PauseVAD()
	}
}

// Resume 恢复VAD检测（TTS播放结束后）
func (vm *VADManager) Resume() {
	vm.mu.Lock()
	defer vm.mu.Unlock()

	if !vm.isActive || !vm.isPaused {
		return
	}

	vm.isPaused = false
	vm.lastResumeTime = time.Now()
	logrus.Debug("🔊 VAD管理器已恢复，设置宽限期")

	// 恢复音频管理器VAD
	if vm.audioManager != nil && vm.audioManager.VAD() != nil {
		vm.audioManager.ResumeVAD()
	}

	// 为唤醒词检测器设置宽限期
	vm.setupWakeWordGracePeriod()
}

// IsPaused 检查VAD是否被暂停
func (vm *VADManager) IsPaused() bool {
	vm.mu.RLock()
	defer vm.mu.RUnlock()
	return vm.isPaused
}

// SetSilenceTimeoutCallback 设置静音超时回调
func (vm *VADManager) SetSilenceTimeoutCallback(callback func()) {
	vm.mu.Lock()
	defer vm.mu.Unlock()
	vm.onSilenceTimeout = callback
}

// handleSilenceDetection 处理静音检测
func (vm *VADManager) handleSilenceDetection() {
	vm.mu.RLock()
	defer vm.mu.RUnlock()

	// 检查是否在宽限期内
	if !vm.lastResumeTime.IsZero() && time.Since(vm.lastResumeTime) < vm.gracePeriod {
		logrus.Debugf("VAD管理器：在宽限期内，跳过静音检测（恢复后%.0fms）", time.Since(vm.lastResumeTime).Milliseconds())
		return
	}

	// 检查是否正在录音
	if vm.recordingController != nil {
		// 使用类型断言检查录音状态
		if recorder, ok := vm.recordingController.(interface{ IsRecording() bool }); ok && recorder.IsRecording() {
			// 检查客户端状态
			if vm.clientInstance != nil && vm.clientInstance.GetState() == client.StateListening {
				logrus.Info("🔇 VAD管理器：检测到静音超时，停止录音")

				// 调用外部回调
				if vm.onSilenceTimeout != nil {
					vm.onSilenceTimeout()
				}

				// 停止VAD管理器
				vm.stopVADManager()
			} else {
				logrus.Debug("VAD管理器：客户端不在监听状态，跳过静音检测")
			}
		} else {
			logrus.Debug("VAD管理器：录音已停止，跳过静音检测")
		}
	}
}

// stopVADManager 停止VAD管理器
func (vm *VADManager) stopVADManager() {
	if !vm.isActive {
		return
	}

	logrus.Info("🛑 停止VAD管理器")
	vm.isActive = false

	// 清除回调
	vm.onSilenceTimeout = nil
}

// setupWakeWordGracePeriod 为唤醒词检测器设置宽限期
func (vm *VADManager) setupWakeWordGracePeriod() {
	// 唤醒词检测器VAD功能已移除，此函数留空
}


// GetSilenceDuration 获取当前静音持续时间
func (vm *VADManager) GetSilenceDuration() time.Duration {
	if vm.audioManager != nil {
		return vm.audioManager.GetVADSilenceDuration()
	}
	return 0
}

// IsVADSpeech 检查是否检测到语音
func (vm *VADManager) IsVADSpeech() bool {
	if vm.audioManager != nil {
		return vm.audioManager.IsVADSpeech()
	}
	return false
}

// IsVADSilence 检查是否为静音
func (vm *VADManager) IsVADSilence() bool {
	if vm.audioManager != nil {
		return vm.audioManager.IsVADSilence()
	}
	return false
}