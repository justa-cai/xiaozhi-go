package audio

import (
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
)

const (
	DefaultSampleRate      = 16000
	DefaultChannelCount    = 1
	DefaultFrameDuration   = 60  // 毫秒
	DefaultFramesPerBuffer = 960 // 60ms at 16kHz
	DefaultBitrate         = 16000
	DefaultComplexity      = 10
	DefaultMaxValue        = 1<<15 - 1
)

// AudioManagerNew 使用新实现的音频管理器
type AudioManagerNew struct {
	recorder          Recorder        // 新的录音器，改为接口
	player            *AudioPlayerNew // 新的播放器
	codec             *OpusCodec      // 编解码器
	initialized       bool            // 初始化标志
	sampleRate        int             // 采样率
	channelCount      int             // 通道数
	frameDuration     int             // 帧持续时间（毫秒）
}

// AudioManagerOptions 音频管理器选项
type AudioManagerOptions struct {
	SampleRate        int    // 采样率
	ChannelCount      int    // 通道数
	FrameDuration     int    // 帧持续时间（毫秒）
	InputDeviceName   string // 输入设备名称（可选）
	OutputDeviceName  string // 输出设备名称（可选）
	UseDefaultDevices bool   // 是否使用默认设备
}

// InitializeAudio 初始化音频系统（Oto无需初始化，直接返回nil）
func InitializeAudio() error {
	return nil
}

// TerminateAudio 终止音频系统（Oto无需终止，直接返回nil）
func TerminateAudio() error {
	return nil
}

// GetAudioDevices 获取音频设备列表（Oto不支持，返回空）
func GetAudioDevices() ([]interface{}, error) {
	return nil, nil
}

// PrintDeviceInfo 打印设备信息（Oto不支持，打印提示）
func PrintDeviceInfo() {
	logrus.Info("Oto不支持枚举音频设备，仅支持默认输出")
}

// NewAudioManagerWithOptions 使用指定选项创建新的音频管理器
func NewAudioManagerWithOptions(options AudioManagerOptions) (*AudioManagerNew, error) {
	// 初始化音频系统
	err := InitializeAudio()
	if err != nil {
		return nil, fmt.Errorf("初始化音频系统失败: %v", err)
	}

	// 使用默认值处理未指定的选项
	if options.SampleRate <= 0 {
		options.SampleRate = DefaultSampleRate
	}
	if options.ChannelCount <= 0 {
		options.ChannelCount = DefaultChannelCount
	}
	if options.FrameDuration <= 0 {
		options.FrameDuration = DefaultFrameDuration
	}

	// 创建编解码器
	codec, err := NewOpusCodec(options.SampleRate, options.ChannelCount)
	if err != nil {
		TerminateAudio()
		return nil, fmt.Errorf("创建Opus编解码器失败: %v", err)
	}

	// 创建录音器（直接用NewRecorder，不再用老的Options/WithOptions）
	recorder := NewRecorder()

	// 创建播放器
	playerOptions := NewPlayerOptions{
		SampleRate:       options.SampleRate,
		ChannelCount:     options.ChannelCount,
		FramesPerBuffer:  (options.SampleRate * options.FrameDuration) / 1000,
		UseDefaultDevice: options.UseDefaultDevices,
		DeviceName:       options.OutputDeviceName,
	}

	player, err := NewAudioPlayerWithOptions(playerOptions, codec)
	if err != nil {
		recorder.Close()
		codec.Close()
		TerminateAudio()
		return nil, fmt.Errorf("创建播放器失败: %v", err)
	}

	return &AudioManagerNew{
		recorder:      recorder,
		player:        player,
		codec:         codec,
		initialized:   true,
		sampleRate:    options.SampleRate,
		channelCount:  options.ChannelCount,
		frameDuration: options.FrameDuration,
	}, nil
}

// NewAudioManager 创建新的音频管理器（使用默认选项）
func NewAudioManager() (*AudioManagerNew, error) {
	options := AudioManagerOptions{
		SampleRate:        DefaultSampleRate,
		ChannelCount:      DefaultChannelCount,
		FrameDuration:     DefaultFrameDuration,
		UseDefaultDevices: true,
	}

	return NewAudioManagerWithOptions(options)
}

// Close 关闭音频管理器并释放资源
func (m *AudioManagerNew) Close() error {
	// 添加一个恢复机制，防止任何异常导致无法正常清理资源
	defer func() {
		if r := recover(); r != nil {
			logrus.Errorf("关闭音频管理器时发生异常: %v", r)
		}
	}()

	if !m.initialized {
		return nil
	}

	// 关闭录音器
	if m.recorder != nil {
		if err := m.recorder.Close(); err != nil {
			logrus.Warnf("关闭录音器失败: %v", err)
		}
	}

	// 关闭播放器
	if m.player != nil {
		if err := m.player.Close(); err != nil {
			logrus.Warnf("关闭播放器失败: %v", err)
		}
	}

	// 关闭编解码器
	if m.codec != nil {
		m.codec.Close()
	}

	// 等待一小段时间，确保所有资源都释放
	time.Sleep(100 * time.Millisecond)

	// 终止PortAudio
	err := TerminateAudio()
	if err != nil {
		logrus.Warnf("终止音频系统失败: %v", err)
	}

	m.initialized = false
	logrus.Debug("音频管理器已关闭")
	return nil
}

// AddAudioDataCallback 添加opus编码后音频数据回调函数
func (m *AudioManagerNew) AddAudioDataCallback(id string, callback func([]byte)) {
	// 设置PCM回调，编码后回调opus数据
	m.recorder.AddPCMDataCallback("opus_"+id, func(pcm []int16, _ int) {
		if m.codec != nil {
			if opus, err := m.codec.Encode(pcm); err == nil {
				callback(opus)
			}
		}
	})
}

// RemoveAudioDataCallback 移除opus编码后音频数据回调函数
func (m *AudioManagerNew) RemoveAudioDataCallback(id string) {
	m.recorder.RemovePCMDataCallback("opus_"+id)
}

// AddPCMDataCallback 添加PCM音频数据回调函数
func (m *AudioManagerNew) AddPCMDataCallback(id string, callback func([]int16, int)) {
	m.recorder.AddPCMDataCallback(id, callback)
}

// This method was replaced by AddPCMDataCallback and RemovePCMDataCallback
// SetPCMDataCallback 设置PCM音频数据回调函数
func (m *AudioManagerNew) SetPCMDataCallback(callback func([]int16, int)) {
	// For backward compatibility, we'll use a default ID
	m.recorder.AddPCMDataCallback("default", callback)
}

// RemovePCMDataCallback 移除PCM音频数据回调函数
func (m *AudioManagerNew) RemovePCMDataCallback(id string) {
	m.recorder.RemovePCMDataCallback(id)
}

// StartRecording 开始录音
func (m *AudioManagerNew) StartRecording() error {
	return m.recorder.StartRecording(m.codec)
}

// StopRecording 停止录音
func (m *AudioManagerNew) StopRecording() error {
	return m.recorder.StopRecording()
}

// StartPlaying 开始播放
func (m *AudioManagerNew) StartPlaying() error {
	return m.player.Start()
}

// StopPlaying 停止播放
func (m *AudioManagerNew) StopPlaying() error {
	return m.player.Stop()
}

// PlayAudio 播放Opus编码的音频数据
func (m *AudioManagerNew) PlayAudio(opusData []byte) {
	m.player.QueueAudio(opusData)
}

// PlayPCMAudio 播放PCM音频数据
func (m *AudioManagerNew) PlayPCMAudio(pcmData []int16) {
	m.player.QueuePCMAudio(pcmData)
}

// IsRecording 检查是否正在录音
func (m *AudioManagerNew) IsRecording() bool {
	return m.recorder.IsRecording()
}

// IsPlaying 检查是否正在播放
func (m *AudioManagerNew) IsPlaying() bool {
	return m.player.IsPlaying()
}

// IsDummyMode 检查播放器是否在哑模式下运行
func (m *AudioManagerNew) IsDummyMode() bool {
	return m.player.IsDummyMode()
}

// GetQueueLength 获取播放队列长度
func (m *AudioManagerNew) GetQueueLength() int {
	return m.player.GetQueueLength()
}

// SampleRate 获取采样率
func (m *AudioManagerNew) SampleRate() int {
	return m.sampleRate
}

// ChannelCount 获取通道数
func (m *AudioManagerNew) ChannelCount() int {
	return m.channelCount
}

// FrameDuration 获取帧持续时间（毫秒）
func (m *AudioManagerNew) FrameDuration() int {
	return m.frameDuration
}

// Player 返回播放器实例
func (m *AudioManagerNew) Player() *AudioPlayerNew {
	return m.player
}

// RecreatePlayer 根据新参数重建播放器（含 Oto Context）
func (m *AudioManagerNew) RecreatePlayer(sampleRate, channelCount, frameDuration int) error {
	if otoInited {
		return fmt.Errorf("Oto Context 已初始化，不能重复创建")
	}
	if m.player != nil {
		m.player.Close()
	}
	codec, err := NewOpusCodec(sampleRate, channelCount)
	if err != nil {
		return err
	}
	options := NewPlayerOptions{
		SampleRate:       sampleRate,
		ChannelCount:     channelCount,
		FramesPerBuffer:  (sampleRate * frameDuration) / 1000,
		UseDefaultDevice: true,
	}
	player, err := NewAudioPlayerWithOptions(options, codec)
	if err != nil {
		return err
	}
	m.player = player
	otoInited = true
	return nil
}
