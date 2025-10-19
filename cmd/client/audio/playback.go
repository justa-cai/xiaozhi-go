package audio

import (
	"time"

	"github.com/sirupsen/logrus"
)

// PlaybackController 音频播放控制器
type PlaybackController struct {
	manager *Manager
}

// NewPlaybackController 创建新的播放控制器
func NewPlaybackController(manager *Manager) *PlaybackController {
	return &PlaybackController{
		manager: manager,
	}
}

// ProcessBinaryAudioData 处理二进制音频数据
func (pc *PlaybackController) ProcessBinaryAudioData(data []byte, verboseLogging bool) {
	if pc.manager == nil || !pc.manager.IsInitialized() {
		logrus.Warn("音频播放器未初始化，无法播放收到的音频数据")
		return
	}

	player := pc.manager.Player()
	if player == nil {
		logrus.Warn("音频播放器为空，无法播放音频数据")
		return
	}

	if verboseLogging {
		logrus.Infof("📥 接收到二进制数据: %d字节", len(data))
	}

	// 检查音频播放器状态
	if !player.IsPlaying() {
		// 播放器未运行，可能是因为刚初始化或之前有错误
		logrus.Debug("音频播放器未运行，尝试启动...")
		if err := player.Start(); err != nil {
			logrus.Errorf("启动音频播放器失败: %v", err)
			return
		}
	}

	// 如果播放器在哑模式下运行，记录一下
	if player.IsDummyMode() && verboseLogging {
		logrus.Debug("音频播放器在哑模式下运行，可能无法实际播放音频")
	}

	// 将Opus编码的音频数据添加到播放队列
	player.QueueAudio(data)

	if verboseLogging {
		logrus.Debugf("已将%d字节Opus编码音频数据添加到播放队列", len(data))
	}
}

// ProcessAudioData 处理音频数据（来自客户端回调）
func (pc *PlaybackController) ProcessAudioData(data []byte, verboseLogging bool) {
	if pc.manager == nil || !pc.manager.IsInitialized() {
		return
	}

	player := pc.manager.Player()
	if player != nil {
		player.QueueAudio(data)
		if player.IsDummyMode() {
			// 如果是哑模式，简单记录一下
			logrus.Debugf("音频在哑模式下处理")
		}
	}
}

// StopAudioPlayback 停止音频播放
func (pc *PlaybackController) StopAudioPlayback() {
	if pc.manager == nil || !pc.manager.IsInitialized() {
		return
	}

	// 先等待500毫秒，给音频播放器一些时间处理缓冲区中的数据
	logrus.Debug("等待500毫秒后停止音频播放...")
	time.Sleep(500 * time.Millisecond)

	// 停止音频播放
	player := pc.manager.Player()
	if player != nil && player.IsPlaying() {
		if err := player.Stop(); err != nil {
			logrus.Errorf("停止音频播放失败: %v", err)
		} else {
			logrus.Info("已停止音频播放")
		}
	}
}

// ReinitializeDecoder 重新初始化Opus解码器
func (pc *PlaybackController) ReinitializeDecoder(sampleRate, channels, frameDuration int) error {
	if pc.manager == nil || !pc.manager.IsInitialized() {
		logrus.Error("音频管理器未初始化，无法重新初始化解码器")
		return ErrNotInitialized
	}

	if sampleRate <= 0 || channels <= 0 || frameDuration <= 0 {
		logrus.Error("无效的音频参数，无法初始化Opus解码器")
		return ErrInvalidConfig
	}

	logrus.Infof("开始重新初始化Opus解码器: sample_rate=%d, channels=%d, frame_duration=%d",
		sampleRate, channels, frameDuration)

	err := pc.manager.RecreatePlayer(sampleRate, channels, frameDuration)
	if err != nil {
		logrus.Warnf("重建播放器失败: %v，尝试使用现有播放器", err)

		// 尝试使用现有播放器
		player := pc.manager.Player()
		if player != nil {
			if !player.IsPlaying() {
				if err := player.Start(); err != nil {
					logrus.Errorf("启动现有播放器失败: %v", err)
					return err
				} else {
					logrus.Info("现有播放器已启动")
				}
			} else {
				logrus.Info("现有播放器已在运行")
			}
		} else {
			logrus.Error("无法获取播放器实例")
			return ErrPlayerNotInitialized
		}
	} else {
		player := pc.manager.Player()
		if player != nil {
			player.Start()
			logrus.Info("已根据服务器参数重建播放器")
		} else {
			logrus.Error("重建播放器后获取播放器失败")
			return ErrPlayerNotInitialized
		}
	}

	return nil
}

// IsPlaying 检查是否正在播放
func (pc *PlaybackController) IsPlaying() bool {
	if pc.manager == nil || !pc.manager.IsInitialized() {
		return false
	}

	player := pc.manager.Player()
	if player != nil {
		return player.IsPlaying()
	}
	return false
}

// IsDummyMode 检查是否为哑模式
func (pc *PlaybackController) IsDummyMode() bool {
	if pc.manager == nil || !pc.manager.IsInitialized() {
		return false
	}

	player := pc.manager.Player()
	if player != nil {
		return player.IsDummyMode()
	}
	return false
}

// Close 关闭播放器
func (pc *PlaybackController) Close() error {
	if pc.manager == nil {
		return ErrManagerNil
	}

	return pc.manager.ClosePlayer()
}