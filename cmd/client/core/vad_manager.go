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
	silenceStartTime time.Time     // 静音开始时间

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
		logrus.Debug("🎯 VAD管理器已经在运行中")
		return nil
	}

	logrus.Info("🎯 启动统一VAD管理器")

	// 重置VAD状态和计时器
	vm.lastResumeTime = time.Now()
	vm.silenceStartTime = time.Time{} // 重置静音开始时间
	vm.isPaused = false
	logrus.Debugf("🔄 VAD状态已重置：恢复时间=%v, 静音开始时间=%v, 暂停状态=%v", vm.lastResumeTime, vm.silenceStartTime, vm.isPaused)

	// 重置底层VAD检测器状态
	if vm.audioManager != nil && vm.audioManager.VAD() != nil {
		vm.audioManager.VAD().Reset()
		logrus.Debug("🔄 VAD检测器状态已重置")
	}

	// 禁用音频管理器的内部VAD超时机制，完全由我们控制
	if vm.audioManager != nil && vm.audioManager.VAD() != nil {
		// 设置我们的回调函数
		vm.audioManager.SetVADCallbacks(
			vm.handleSpeechDetection, // 语音开始回调，重置静音计时器
			vm.handleSilenceDetection,
		)
		logrus.Info("✅ 音频管理器VAD回调已设置")
	} else {
		logrus.Warn("⚠️ 音频管理器或VAD检测器为空，无法设置回调")
	}

	vm.isActive = true
	logrus.Infof("✅ VAD管理器启动成功，静音阈值: %v, 宽限期: %v", vm.silenceThreshold, vm.gracePeriod)
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

// handleSpeechDetection 处理语音检测（重置静音计时器）
func (vm *VADManager) handleSpeechDetection() {
	vm.mu.Lock()
	defer vm.mu.Unlock()

	// 检查VAD管理器是否处于活动状态
	if !vm.isActive {
		logrus.Debug("VAD管理器：不在活动状态，跳过语音检测")
		return
	}

	// 检查是否在宽限期内
	if !vm.lastResumeTime.IsZero() && time.Since(vm.lastResumeTime) < vm.gracePeriod {
		logrus.Debugf("VAD管理器：在宽限期内，跳过语音检测（恢复后%.0fms）", time.Since(vm.lastResumeTime).Milliseconds())
		return
	}

	// 重置静音开始时间
	if !vm.silenceStartTime.IsZero() {
		silenceDuration := time.Since(vm.silenceStartTime)
		logrus.Debugf("🔊 VAD管理器：检测到语音，重置静音计时器（之前静音了%.1f秒）", silenceDuration.Seconds())
		vm.silenceStartTime = time.Time{}
	} else {
		logrus.Debug("🔊 VAD管理器：检测到语音，静音计时器已经重置")
	}
}

// handleSilenceDetection 处理静音检测
func (vm *VADManager) handleSilenceDetection() {
	vm.mu.Lock()
	defer vm.mu.Unlock()

	// 检查VAD管理器是否处于活动状态
	if !vm.isActive {
		logrus.Debug("VAD管理器：不在活动状态，跳过静音检测")
		return
	}

	// 检查是否在宽限期内
	if !vm.lastResumeTime.IsZero() && time.Since(vm.lastResumeTime) < vm.gracePeriod {
		logrus.Debugf("VAD管理器：在宽限期内，跳过静音检测（恢复后%.0fms）", time.Since(vm.lastResumeTime).Milliseconds())
		return
	}

	// 检查客户端状态
	if vm.clientInstance == nil {
		logrus.Warn("VAD管理器：客户端实例为空，无法检测状态")
		return
	}

	currentState := vm.clientInstance.GetState()
	if currentState != client.StateListening {
		logrus.Debugf("VAD管理器：客户端状态为 %s，不在监听状态，跳过静音检测", currentState)
		return
	}

	// 检查录音状态（使用多种方式验证）
	isRecording := false
	if vm.recordingController != nil {
		// 方法1：使用录音控制器的IsRecording方法
		if recorder, ok := vm.recordingController.(interface{ IsRecording() bool }); ok {
			isRecording = recorder.IsRecording()
		}

		// 方法2：如果方法1返回false但客户端状态是listening，也认为在录音
		if !isRecording && currentState == client.StateListening {
			logrus.Debug("VAD管理器：录音控制器显示未录音，但客户端状态为listening，强制启用录音检测")
			isRecording = true
		}
	}

	if !isRecording {
		logrus.Debug("VAD管理器：录音未在进行中，跳过静音检测")
		return
	}

	// 记录静音开始时间（如果还没有开始）
	if vm.silenceStartTime.IsZero() {
		vm.silenceStartTime = time.Now()
		logrus.Debugf("🔇 VAD管理器：开始记录静音时间，开始时间=%v", vm.silenceStartTime)
		return
	}

	// 检查是否已经达到静音阈值
	silenceDuration := time.Since(vm.silenceStartTime)
	if silenceDuration >= vm.silenceThreshold {
		logrus.Infof("🔇 VAD管理器：检测到%.1f秒静音超时，开始停止录音", silenceDuration.Seconds())

		// 调用外部回调
		if vm.onSilenceTimeout != nil {
			logrus.Debug("VAD管理器：调用静音超时回调")
			vm.onSilenceTimeout()
		} else {
			logrus.Warn("VAD管理器：静音超时回调为空")
		}

		// 停止VAD管理器
		vm.stopVADManager()
	} else {
		// 还未达到阈值，继续等待
		logrus.Debugf("🔇 VAD管理器：静音持续%.1f秒，还需%.1f秒达到阈值",
			silenceDuration.Seconds(),
			(vm.silenceThreshold - silenceDuration).Seconds())
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