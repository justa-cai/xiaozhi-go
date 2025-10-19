package audio

import (
	"time"

	"github.com/justa-cai/xiaozhi-go/internal/audio"
	"github.com/sirupsen/logrus"
)

// Manager 音频管理器包装器
type Manager struct {
	audioManager   *audio.AudioManagerNew
	vadConfig      *audio.VADConfig
	isInitialized  bool
	config         AudioConfig
}

// AudioConfig 音频配置
type AudioConfig struct {
	SampleRate                  int
	ChannelCount                int
	FrameDuration               int
	UseDefaultDevices           bool
	EnableVAD                   bool
	SilenceTimeoutMs            int
	AutoInteractionSilenceThreshold int // 自动交互模式的静音阈值（秒）
}

// NewManager 创建新的音频管理器
func NewManager(config AudioConfig) *Manager {
	return &Manager{
		config:        config,
		isInitialized: false,
	}
}

// Initialize 初始化音频系统
func (m *Manager) Initialize() error {
	if m.isInitialized {
		return nil
	}

	logrus.Debug("开始初始化音频系统...")

	// 创建音频管理器选项
	audioOptions := audio.AudioManagerOptions{
		SampleRate:        m.config.SampleRate,
		ChannelCount:      m.config.ChannelCount,
		FrameDuration:     m.config.FrameDuration,
		UseDefaultDevices: m.config.UseDefaultDevices,
	}

	// 只有启用VAD时才创建VAD配置
	if m.config.EnableVAD {
		vadConfig := audio.NewVADConfig()
		vadConfig.Enabled = true
		vadConfig.Threshold = 0.5
		vadConfig.MinSilenceDuration = 0.5
		vadConfig.MinSpeechDuration = 0.25
		vadConfig.MaxSpeechDuration = 10.0
		vadConfig.WindowSize = 512
		vadConfig.SampleRate = m.config.SampleRate
		// 在自动交互模式下，禁用VAD的内部超时机制，完全依赖外部回调控制
		if m.config.AutoInteractionSilenceThreshold > 0 {
			vadConfig.SilenceTimeout = 0 // 设置为0表示禁用内部超时，只依赖回调
			logrus.Infof("🔧 自动交互模式：禁用VAD内部超时机制，静音检测由外部回调控制（期望%.1fs）", float64(m.config.AutoInteractionSilenceThreshold))
		} else {
			vadConfig.SilenceTimeout = time.Duration(m.config.SilenceTimeoutMs) * time.Millisecond
			logrus.Infof("VAD静音超时设置为: %dms（默认模式）", m.config.SilenceTimeoutMs)
		}
		vadConfig.Debug = false // 可以从配置传入
		audioOptions.VADConfig = vadConfig
		m.vadConfig = vadConfig
		logrus.Info("高级VAD语音检测已启用")
	} else {
		logrus.Info("使用简单能量阈值检测，高级VAD功能已禁用")
	}

	// 初始化音频管理器
	audioManager, err := audio.NewAudioManagerWithOptions(audioOptions)
	if err != nil {
		logrus.Warnf("初始化音频管理器失败: %v，将无法录音和播放", err)
		return err
	}

	m.audioManager = audioManager
	m.isInitialized = true

	logrus.Debug("音频管理器初始化成功")
	if m.config.EnableVAD && m.audioManager.VAD() != nil {
		logrus.Info("高级VAD语音检测器已初始化并集成到音频管理器")
	} else if m.config.EnableVAD {
		logrus.Warn("VAD功能已启用但检测器未初始化，将使用简单能量阈值检测")
	}

	logrus.Info("音频系统初始化完成")
	return nil
}

// GetAudioManager 获取底层音频管理器
func (m *Manager) GetAudioManager() *audio.AudioManagerNew {
	return m.audioManager
}

// Player 获取音频播放器
func (m *Manager) Player() *audio.AudioPlayerNew {
	if m.audioManager != nil {
		return m.audioManager.Player()
	}
	return nil
}

// VAD 获取VAD检测器
func (m *Manager) VAD() *audio.VADDetector {
	if m.audioManager != nil {
		return m.audioManager.VAD()
	}
	return nil
}

// IsRecording 检查是否正在录音
func (m *Manager) IsRecording() bool {
	if m.audioManager != nil {
		return m.audioManager.IsRecording()
	}
	return false
}

// StartRecording 开始录音
func (m *Manager) StartRecording() error {
	if !m.isInitialized {
		return ErrNotInitialized
	}
	if m.audioManager == nil {
		return ErrManagerNil
	}
	return m.audioManager.StartRecording()
}

// StopRecording 停止录音
func (m *Manager) StopRecording() error {
	if !m.isInitialized {
		return ErrNotInitialized
	}
	if m.audioManager == nil {
		return ErrManagerNil
	}
	return m.audioManager.StopRecording()
}

// ProcessVADAudio 处理VAD音频数据
func (m *Manager) ProcessVADAudio(data []int16) error {
	if m.audioManager == nil {
		return ErrManagerNil
	}
	return m.audioManager.ProcessVADAudio(data)
}

// IsVADSpeech 检查VAD是否检测到语音
func (m *Manager) IsVADSpeech() bool {
	if m.audioManager == nil {
		return false
	}
	return m.audioManager.IsVADSpeech()
}

// IsVADSilence 检查VAD是否检测到静音
func (m *Manager) IsVADSilence() bool {
	if m.audioManager == nil {
		return false
	}
	return m.audioManager.IsVADSilence()
}

// SetVADCallbacks 设置VAD回调函数
func (m *Manager) SetVADCallbacks(onSpeech, onSilence func()) {
	if m.audioManager != nil && m.audioManager.VAD() != nil {
		m.audioManager.SetVADCallbacks(onSpeech, onSilence)
	}
}

// GetVADSilenceDuration 获取VAD静音持续时间
func (m *Manager) GetVADSilenceDuration() time.Duration {
	if m.audioManager == nil {
		return 0
	}
	return m.audioManager.GetVADSilenceDuration()
}

// PauseVAD 暂停VAD检测（用于TTS播放期间）
func (m *Manager) PauseVAD() {
	if m.audioManager != nil && m.audioManager.VAD() != nil {
		logrus.Debug("🔇 VAD检测已暂停（TTS播放期间）")
		m.audioManager.VAD().Pause()
	}
}

// ResumeVAD 恢复VAD检测（TTS播放结束后）
func (m *Manager) ResumeVAD() {
	if m.audioManager != nil && m.audioManager.VAD() != nil {
		logrus.Debug("🔊 VAD检测已恢复（TTS播放结束）")
		m.audioManager.VAD().Resume()
	}
}

// AddAudioDataCallback 添加音频数据回调
func (m *Manager) AddAudioDataCallback(name string, callback func([]byte)) {
	if m.audioManager != nil {
		m.audioManager.AddAudioDataCallback(name, callback)
	}
}

// AddPCMDataCallback 添加PCM数据回调
func (m *Manager) AddPCMDataCallback(name string, callback func([]int16, int)) {
	if m.audioManager != nil {
		m.audioManager.AddPCMDataCallback(name, callback)
	}
}

// RecreatePlayer 重建播放器
func (m *Manager) RecreatePlayer(sampleRate, channels, frameDuration int) error {
	if m.audioManager == nil {
		return ErrManagerNil
	}
	return m.audioManager.RecreatePlayer(sampleRate, channels, frameDuration)
}

// Close 关闭音频管理器
func (m *Manager) Close() error {
	if m.audioManager != nil {
		return m.audioManager.Close()
	}
	return nil
}

// ClosePlayer 关闭播放器
func (m *Manager) ClosePlayer() error {
	if m.audioManager != nil && m.audioManager.Player() != nil {
		return m.audioManager.Player().Close()
	}
	return nil
}

// IsInitialized 检查是否已初始化
func (m *Manager) IsInitialized() bool {
	return m.isInitialized
}

// GetConfig 获取配置
func (m *Manager) GetConfig() AudioConfig {
	return m.config
}