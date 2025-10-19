package audio

import (
	"math"

	"github.com/justa-cai/xiaozhi-go/internal/audio"
	"github.com/sirupsen/logrus"
)

// SoundEffectsManager 音效管理器
type SoundEffectsManager struct {
	player   *audio.AudioPlayerNew
	isActive bool
}

// NewSoundEffectsManager 创建新的音效管理器
func NewSoundEffectsManager(player *audio.AudioPlayerNew) *SoundEffectsManager {
	return &SoundEffectsManager{
		player:   player,
		isActive: false,
	}
}

// IsActive 检查音效管理器是否激活
func (sem *SoundEffectsManager) IsActive() bool {
	return sem.isActive && sem.player != nil
}

// Activate 激活音效管理器
func (sem *SoundEffectsManager) Activate() {
	if sem.player != nil {
		sem.isActive = true
		logrus.Debug("音效管理器已激活")
	}
}

// Deactivate 停用音效管理器
func (sem *SoundEffectsManager) Deactivate() {
	sem.isActive = false
	logrus.Debug("音效管理器已停用")
}

// PlayDengSound 播放"deng"提示音
func (sem *SoundEffectsManager) PlayDengSound() {
	if !sem.IsActive() {
		logrus.Debug("音效管理器未激活，跳过播放提示音")
		return
	}

	if sem.player == nil {
		logrus.Error("音频播放器为空，无法播放提示音")
		return
	}

	go func() {
		logrus.Info("🔔 播放deng提示音...")

		// 检查播放器状态
		logrus.Infof("播放器状态: 正在播放=%v, 哑模式=%v, 队列长度=%d",
			sem.player.IsPlaying(), sem.player.IsDummyMode(), sem.player.GetQueueLength())

		// 生成deng声音 - 一个短促的 beep 音
		dengAudio := sem.generateDengSound()
		logrus.Infof("生成了%d个PCM采样点的deng提示音", len(dengAudio))

		// 播放声音
		sem.player.QueuePCMAudio(dengAudio)
		logrus.Info("✅ deng提示音已添加到播放队列")
	}()
}

// generateDengSound 生成deng声音的PCM数据
func (sem *SoundEffectsManager) generateDengSound() []int16 {
	// 音频参数
	sampleRate := 16000  // 采样率
	duration := 0.3      // 持续时间（秒）- 增加到0.3秒
	frequency := 1000.0  // 频率（Hz）- 提高到1000Hz，更明显

	// 计算样本数
	numSamples := int(float64(sampleRate) * duration)
	pcmData := make([]int16, numSamples)

	// 生成正弦波
	for i := 0; i < numSamples; i++ {
		t := float64(i) / float64(sampleRate)

		// 使用更平缓的包络，让声音持续更久
		var envelope float64
		if t < 0.05 {
			// 前50ms渐入
			envelope = t / 0.05
		} else if t > duration-0.05 {
			// 后50ms渐出
			envelope = (duration - t) / 0.05
		} else {
			// 中间部分保持最大音量
			envelope = 1.0
		}

		// 生成正弦波，添加一些谐波使声音更丰富
		sample := math.Sin(2 * math.Pi * frequency * t) * 0.7 +
			math.Sin(4 * math.Pi * frequency * t) * 0.2 + // 二次谐波
			math.Sin(6 * math.Pi * frequency * t) * 0.1   // 三次谐波

		sample = sample * envelope

		// 转换为16位PCM，提高音量到约80%最大音量
		pcmData[i] = int16(sample * 26214)
	}

	return pcmData
}

// PlayNotificationSound 播放通知声音（可用于其他场景）
func (sem *SoundEffectsManager) PlayNotificationSound() {
	if !sem.IsActive() {
		return
	}

	go func() {
		logrus.Debug("🔔 播放通知提示音...")

		// 生成通知声音 - 两个短促的 beep
		notificationAudio := sem.generateNotificationSound()

		sem.player.QueuePCMAudio(notificationAudio)
		logrus.Debug("✅ 通知提示音已添加到播放队列")
	}()
}

// generateNotificationSound 生成通知声音的PCM数据
func (sem *SoundEffectsManager) generateNotificationSound() []int16 {
	sampleRate := 16000
	beepDuration := 0.1    // 每个 beep 持续时间
	beepFrequency := 1000.0 // beep 频率
	pauseDuration := 0.05   // beep 之间的暂停时间

	// 计算总样本数
	totalDuration := beepDuration + pauseDuration + beepDuration
	numSamples := int(float64(sampleRate) * totalDuration)
	pcmData := make([]int16, numSamples)

	sampleIndex := 0

	// 第一个 beep
	beep1Samples := int(float64(sampleRate) * beepDuration)
	for i := 0; i < beep1Samples; i++ {
		if sampleIndex >= numSamples {
			break
		}
		t := float64(i) / float64(sampleRate)
		envelope := math.Exp(-t * 10.0)
		sample := math.Sin(2 * math.Pi * beepFrequency * t) * envelope
		pcmData[sampleIndex] = int16(sample * 16384)
		sampleIndex++
	}

	// 暂停
	pauseSamples := int(float64(sampleRate) * pauseDuration)
	for i := 0; i < pauseSamples; i++ {
		if sampleIndex >= numSamples {
			break
		}
		pcmData[sampleIndex] = 0
		sampleIndex++
	}

	// 第二个 beep
	beep2Samples := int(float64(sampleRate) * beepDuration)
	for i := 0; i < beep2Samples; i++ {
		if sampleIndex >= numSamples {
			break
		}
		t := float64(i) / float64(sampleRate)
		envelope := math.Exp(-t * 10.0)
		sample := math.Sin(2 * math.Pi * beepFrequency * t) * envelope
		pcmData[sampleIndex] = int16(sample * 16384)
		sampleIndex++
	}

	return pcmData
}

// Close 关闭音效管理器
func (sem *SoundEffectsManager) Close() {
	sem.Deactivate()
	sem.player = nil
	logrus.Debug("音效管理器已关闭")
}