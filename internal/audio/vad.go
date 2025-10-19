package audio

import (
	"fmt"
	"os"
	"sync"
	"time"

	sherpa "github.com/k2-fsa/sherpa-onnx-go/sherpa_onnx"
	"github.com/sirupsen/logrus"
)

// VADDetector 语音活动检测器
type VADDetector struct {
	detector   *sherpa.VoiceActivityDetector
	buffer     *sherpa.CircularBuffer
	config     sherpa.VadModelConfig
	windowSize int
	sampleRate int

	// 状态管理
	mu          sync.Mutex
	isRunning   bool
	onSpeech    func()    // 检测到语音时的回调
	onSilence   func()    // 检测到静音时的回调

	// 静音检测状态
	silenceStartTime time.Time
	isSilence       bool
	silenceTimeout time.Duration

	// VAD暂停状态（用于TTS播放期间）
	isPaused bool

	// VAD恢复时间（用于延迟启动检测）
	lastResumeTime time.Time

	// VAD回调宽限期（防止TTS结束后立即触发）
	callbackGracePeriod time.Duration
}

// VADConfig VAD配置
type VADConfig struct {
	Enabled          bool
	ModelPath        string   // VAD模型路径
	Threshold        float32  // 检测阈值 (0.0-1.0)
	MinSilenceDuration float32  // 最小静音持续时间(秒)
	MinSpeechDuration float32  // 最小语音持续时间(秒)
	MaxSpeechDuration float32  // 最大语音持续时间(秒)
	WindowSize        int      // 窗口大小
	SampleRate       int      // 采样率
	SilenceTimeout    time.Duration // 静音超时时间
	Debug            bool     // 调试模式
}

// VADState VAD状态
type VADState int

const (
	VADStateIdle VADState = iota
	VADStateSpeech
	VADStateSilence
)

// NewVADConfig 创建默认VAD配置
func NewVADConfig() *VADConfig {
	return &VADConfig{
		Enabled:           true,
		ModelPath:         "./models/silero_vad.onnx",
		Threshold:         0.5,
		MinSilenceDuration: 0.5,
		MinSpeechDuration: 0.25,
		MaxSpeechDuration: 10.0,
		WindowSize:       512,
		SampleRate:        16000,
		SilenceTimeout:    500 * time.Millisecond,
		Debug:            false,
	}
}

// NewVADDetector 创建VAD检测器
func NewVADDetector(config *VADConfig, onSpeech func(), onSilence func()) (*VADDetector, error) {
	if config == nil {
		config = NewVADConfig()
	}

	if !config.Enabled {
		logrus.Info("VAD检测器已禁用")
		return &VADDetector{
			config:          sherpa.VadModelConfig{},
			windowSize:      0,
			sampleRate:     0,
			silenceTimeout:  config.SilenceTimeout,
		}, nil
	}

	// 检查模型文件是否存在
	if !fileExists(config.ModelPath) {
		// 尝试查找模型文件
		alternativePaths := []string{
			"./models/" + config.ModelPath,
			"./" + config.ModelPath,
		}

		found := false
		for _, path := range alternativePaths {
			if fileExists(path) {
				config.ModelPath = path
				found = true
				break
			}
		}

		if !found {
			return nil, fmt.Errorf("VAD模型文件不存在: %s", config.ModelPath)
		}
	}

	logrus.Infof("使用VAD模型: %s", config.ModelPath)

	// 设置sherpa VAD配置
	vadConfig := sherpa.VadModelConfig{}
	if fileExists(config.ModelPath) {
		vadConfig.SileroVad.Model = config.ModelPath
		vadConfig.SileroVad.Threshold = config.Threshold
		vadConfig.SileroVad.MinSilenceDuration = config.MinSilenceDuration
		vadConfig.SileroVad.MinSpeechDuration = config.MinSpeechDuration
		vadConfig.SileroVad.MaxSpeechDuration = config.MaxSpeechDuration
		vadConfig.SileroVad.WindowSize = config.WindowSize  // Use config.WindowSize directly (not int32)
		vadConfig.SampleRate = config.SampleRate
		vadConfig.NumThreads = 1
		vadConfig.Provider = "cpu"
		vadConfig.Debug = 0
		if config.Debug {
			vadConfig.Debug = 1
		}
	}

	windowSize := config.WindowSize  // Use config.WindowSize directly to avoid type conversion issues
	bufferSizeInSeconds := float32(5.0) // 5秒缓冲

	// 创建VAD实例
	vad := sherpa.NewVoiceActivityDetector(&vadConfig, bufferSizeInSeconds)
	if vad == nil {
		return nil, fmt.Errorf("创建VAD检测器失败")
	}

	// 创建环形缓冲区
	buffer := sherpa.NewCircularBuffer(10 * int(vadConfig.SampleRate))
	if buffer == nil {
		sherpa.DeleteVoiceActivityDetector(vad)
		return nil, fmt.Errorf("创建VAD缓冲区失败")
	}

	detector := &VADDetector{
		detector:        vad,
		buffer:          buffer,
		config:          vadConfig,
		windowSize:      windowSize,
		sampleRate:      vadConfig.SampleRate,
		onSpeech:        onSpeech,
		onSilence:       onSilence,
		silenceTimeout:  config.SilenceTimeout,
	}

	return detector, nil
}

// Start 启动VAD检测
func (v *VADDetector) Start() error {
	v.mu.Lock()
	defer v.mu.Unlock()

	if v.isRunning {
		return fmt.Errorf("VAD检测器已在运行")
	}

	if v.detector == nil {
		return fmt.Errorf("VAD检测器未初始化")
	}

	v.isRunning = true
	logrus.Info("VAD检测器已启动")
	return nil
}

// Stop 停止VAD检测
func (v *VADDetector) Stop() error {
	v.mu.Lock()
	defer v.mu.Unlock()

	if !v.isRunning {
		return nil
	}

	v.isRunning = false
	logrus.Info("VAD检测器已停止")
	return nil
}

// IsRunning 检查VAD是否在运行
func (v *VADDetector) IsRunning() bool {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.isRunning
}

// ProcessAudioData 处理音频数据
func (v *VADDetector) ProcessAudioData(samples []int16) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	if !v.isRunning || v.detector == nil || v.buffer == nil {
		return nil
	}

	// 如果VAD被暂停（TTS播放期间），跳过处理
	if v.isPaused {
		return nil
	}

	// 检查是否在VAD恢复的宽限期内（避免TTS播放结束后的残留音频影响检测）
	if !v.lastResumeTime.IsZero() {
		timeSinceResume := time.Since(v.lastResumeTime)
		if timeSinceResume < 500*time.Millisecond {
			// 在恢复后的500ms宽限期内，只处理音频但不进行静音检测
			logrus.Debugf("VAD在宽限期内，已处理音频但跳过静音检测（恢复后%.0fms）", timeSinceResume.Milliseconds())

			// 仍然需要将音频数据添加到缓冲区并处理，保持VAD状态同步
			floatSamples := make([]float32, len(samples))
			for i, sample := range samples {
				floatSamples[i] = float32(sample) / 32768.0
			}
			v.buffer.Push(floatSamples)

			// 处理音频窗口但不触发静音检测回调
			for int(v.buffer.Size()) >= v.windowSize {
				head := v.buffer.Head()
				windowSamples := v.buffer.Get(int(head), v.windowSize)
				v.buffer.Pop(v.windowSize)

				// 输入到VAD检测器
				v.detector.AcceptWaveform(windowSamples)

				// 检测语音活动并更新内部状态，但不触发回调
				isSpeech := v.detector.IsSpeech()

				if isSpeech && v.isSilence {
					// 从静音状态切换到语音状态
					v.isSilence = false
					v.silenceStartTime = time.Time{}
					logrus.Debugf("VAD宽限期内：检测到语音活动，重置静音状态")
				} else if !isSpeech && !v.isSilence {
					// 从语音状态切换到静音状态，但在宽限期内不触发回调
					v.isSilence = true
					v.silenceStartTime = time.Now()
					logrus.Debugf("VAD宽限期内：检测到静音，开始记录静音时间但不触发回调")
				}

				// 处理语音段（仅用于清理VAD内部状态）
				for !v.detector.IsEmpty() {
					speechSegment := v.detector.Front()
					v.detector.Pop()
					duration := float32(len(speechSegment.Samples)) / float32(v.sampleRate)
					logrus.Debugf("VAD宽限期内：处理语音段，持续时间 %.2f 秒", duration)
				}
			}

			return nil
		}
	}

	// 转换音频数据格式
	floatSamples := make([]float32, len(samples))
	for i, sample := range samples {
		floatSamples[i] = float32(sample) / 32768.0
	}

	// 添加到缓冲区
	v.buffer.Push(floatSamples)

	// 处理音频窗口
	for int(v.buffer.Size()) >= v.windowSize {
		head := v.buffer.Head()
		windowSamples := v.buffer.Get(int(head), v.windowSize)
		v.buffer.Pop(v.windowSize)

		// 输入到VAD检测器
		v.detector.AcceptWaveform(windowSamples)

	// 检测语音活动
		isSpeech := v.detector.IsSpeech()

		if isSpeech && v.isSilence {
			// 从静音状态切换到语音状态
			v.isSilence = false
			v.silenceStartTime = time.Time{}
			logrus.Debugf("VAD: 检测到语音活动，从静音切换到语音状态")

			if v.onSpeech != nil {
				v.onSpeech()
			}
		} else if !isSpeech && !v.isSilence {
			// 从语音状态切换到静音状态
			v.isSilence = true
			v.silenceStartTime = time.Now()
			logrus.Debugf("VAD: 检测到静音，从语音切换到静音状态")

			// 检查是否在回调宽限期内（防止TTS结束后立即触发）
			if !v.lastResumeTime.IsZero() && time.Since(v.lastResumeTime) < v.callbackGracePeriod {
				logrus.Debugf("VAD: 在回调宽限期内，跳过静音回调（恢复后%.0fms）", time.Since(v.lastResumeTime).Milliseconds())
			} else if v.onSilence != nil {
				v.onSilence()
			}
		} else if !isSpeech && v.isSilence {
			// 持续静音状态，检查是否超时
			silenceDuration := time.Since(v.silenceStartTime)
			if silenceDuration >= v.silenceTimeout {
				logrus.Debugf("VAD: 静音持续时间 %.2f 秒，超过阈值 %.2f 秒",
					silenceDuration.Seconds(), v.silenceTimeout.Seconds())

				// 检查是否在回调宽限期内（防止TTS结束后立即触发）
				if !v.lastResumeTime.IsZero() && time.Since(v.lastResumeTime) < v.callbackGracePeriod {
					logrus.Debugf("VAD: 在回调宽限期内，跳过静音超时回调（恢复后%.0fms）", time.Since(v.lastResumeTime).Milliseconds())
				} else if v.onSilence != nil {
					v.onSilence()
				}
			}
		}

		// 处理语音段
		for !v.detector.IsEmpty() {
			speechSegment := v.detector.Front()
			v.detector.Pop()

			duration := float32(len(speechSegment.Samples)) / float32(v.sampleRate)
			logrus.Debugf("VAD: 处理语音段，持续时间 %.2f 秒", duration)
		}
	}

	return nil
}

// IsSpeech 当前是否检测到语音
func (v *VADDetector) IsSpeech() bool {
	v.mu.Lock()
	defer v.mu.Unlock()

	if v.detector != nil {
		return v.detector.IsSpeech()
	}
	return false
}

// IsSilence 当前是否为静音
func (v *VADDetector) IsSilence() bool {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.isSilence
}

// GetSilenceDuration 获取当前静音持续时间
func (v *VADDetector) GetSilenceDuration() time.Duration {
	v.mu.Lock()
	defer v.mu.Unlock()

	if v.isSilence && !v.silenceStartTime.IsZero() {
		return time.Since(v.silenceStartTime)
	}
	return 0
}

// Reset 重置VAD状态
func (v *VADDetector) Reset() {
	v.mu.Lock()
	defer v.mu.Unlock()

	v.isSilence = false
	v.silenceStartTime = time.Time{}

	if v.detector != nil {
		// 重置VAD检测器状态
		for !v.detector.IsEmpty() {
			v.detector.Pop()
		}
	}

	if v.buffer != nil {
		// 清空缓冲区
		for v.buffer.Size() > 0 {
			v.buffer.Pop(v.buffer.Size())
		}
	}

	logrus.Debug("VAD状态已重置")
}

// SetCallbacks 设置回调函数
func (v *VADDetector) SetCallbacks(onSpeech, onSilence func()) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.onSpeech = onSpeech
	v.onSilence = onSilence
}

// Cleanup 清理资源
func (v *VADDetector) Cleanup() {
	v.mu.Lock()
	defer v.mu.Unlock()

	if v.detector != nil {
		sherpa.DeleteVoiceActivityDetector(v.detector)
		v.detector = nil
	}

	if v.buffer != nil {
		sherpa.DeleteCircularBuffer(v.buffer)
		v.buffer = nil
	}

	v.isRunning = false
	logrus.Debug("VAD资源已清理")
}

// Pause 暂停VAD检测（用于TTS播放期间）
func (v *VADDetector) Pause() {
	v.mu.Lock()
	defer v.mu.Unlock()

	if !v.isPaused {
		v.isPaused = true
		logrus.Debug("🔇 VAD检测已暂停（TTS播放期间）")
	}
}

// Resume 恢复VAD检测（TTS播放结束后）
func (v *VADDetector) Resume() {
	v.mu.Lock()
	defer v.mu.Unlock()

	if v.isPaused {
		v.isPaused = false
		// 重置静音状态，避免TTS播放期间的静音影响检测
		v.isSilence = false
		v.silenceStartTime = time.Time{}

	// 清空VAD检测器缓冲区，避免TTS播放期间的音频数据影响检测
		if v.detector != nil {
			for !v.detector.IsEmpty() {
				v.detector.Pop()
			}
		}
		if v.buffer != nil {
			for v.buffer.Size() > 0 {
				v.buffer.Pop(v.buffer.Size())
			}
		}

		// 添加恢复时间戳和回调宽限期，用于实现延迟启动检测
		v.lastResumeTime = time.Now()
		v.callbackGracePeriod = 1 * time.Second // 设置1秒的回调宽限期
		logrus.Debug("🔊 VAD检测已恢复（TTS播放结束），已清空缓冲区，设置1秒回调宽限期")
	}
}

// IsPaused 检查VAD是否被暂停
func (v *VADDetector) IsPaused() bool {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.isPaused
}

// fileExists 检查文件是否存在
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}