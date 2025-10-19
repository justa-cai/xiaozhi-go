package core

import (
	"errors"
	"fmt"
	"time"

	"github.com/justa-cai/xiaozhi-go/cmd/client/audio"
	"github.com/justa-cai/xiaozhi-go/cmd/client/config"
	"github.com/justa-cai/xiaozhi-go/cmd/client/device"
	"github.com/justa-cai/xiaozhi-go/cmd/client/interaction"
	"github.com/justa-cai/xiaozhi-go/cmd/client/network"
	"github.com/justa-cai/xiaozhi-go/cmd/client/utils"
	"github.com/justa-cai/xiaozhi-go/internal/client"
	"github.com/sirupsen/logrus"
)

// Application 应用程序主体
type Application struct {
	flags                 *config.Flags
	deviceInfo            *device.DeviceInfo
	activationManager     *device.ActivationManager
	audioManager          *audio.Manager
	recordingController   *audio.RecordingController
	playbackController    *audio.PlaybackController
	connectionManager     *network.ConnectionManager
	inputManager          *interaction.InputManager
	wakeWordManager       *interaction.WakeWordManager
	vadManager            *VADManager // 统一的VAD管理器
	clientInstance        *client.Client
	exitManager           *utils.ExitManager
	isRunning             bool
	autoInteractionActive bool // Track if auto interaction is currently active to prevent loops
}

// NewApplication 创建新的应用程序实例
func NewApplication() *Application {
	return &Application{
		exitManager: utils.GetGlobalExitManager(),
		isRunning:   false,
	}
}

// Initialize 初始化应用程序
func (app *Application) Initialize() error {
	// 解析命令行参数
	app.flags = config.ParseFlags()
	if err := app.flags.Validate(); err != nil {
		return err
	}

	// 设置日志系统
	config.SetupLoggerFromConfig(app.flags)

	// 初始化设备信息
	app.deviceInfo = device.NewDeviceInfo(
		app.flags.DeviceID,
		app.flags.BoardType,
		app.flags.AppVersion,
	)
	app.deviceInfo.LogInfo()

	// 初始化激活管理器
	app.activationManager = device.NewActivationManager(app.deviceInfo)

	// 检查设备激活状态
	if !app.flags.ActivateOnly {
		activated, err := app.activationManager.CheckActivationStatus()
		if err != nil {
			logrus.Errorf("检查设备激活状态失败: %v", err)
			return err
		}
		if !activated {
			logrus.Error("设备未激活，请先激活设备")
			return ErrDeviceNotActivated
		}
	}

	// 初始化音频系统
	audioConfig := app.flags.GetAudioConfig()
	internalAudioConfig := audio.AudioConfig{
		SampleRate:                     audioConfig.SampleRate,
		ChannelCount:                   audioConfig.ChannelCount,
		FrameDuration:                  audioConfig.FrameDuration,
		UseDefaultDevices:              audioConfig.UseDefaultDevices,
		EnableVAD:                      audioConfig.EnableVAD,
		SilenceTimeoutMs:               audioConfig.SilenceTimeoutMs,
		AutoInteractionSilenceThreshold: app.flags.AutoInteractionSilenceThreshold,
	}
	app.audioManager = audio.NewManager(internalAudioConfig)
	if err := app.audioManager.Initialize(); err != nil {
		logrus.Warnf("音频系统初始化失败: %v", err)
	}

	// 初始化录音和播放控制器
	app.recordingController = audio.NewRecordingController(app.audioManager)
	app.playbackController = audio.NewPlaybackController(app.audioManager)

	// 初始化统一的VAD管理器
	if app.flags.EnableVAD {
		vadConfig := VADConfig{
			SilenceThreshold: time.Duration(app.flags.AutoInteractionSilenceThreshold) * time.Second,
			EnableWakeWord:   app.flags.EnableWakeWord,
			GracePeriod:      1 * time.Second, // 1秒宽限期
		}
		app.vadManager = NewVADManager(
			app.audioManager,
			app.wakeWordManager,
			app.recordingController,
			vadConfig,
		)
		logrus.Info("🎯 统一VAD管理器已初始化")
	}

	// 初始化网络连接管理器
	connectionConfig := network.ConnectionConfig{
		ServerURL:         app.flags.ServerURL,
		Token:             app.flags.Token,
		DeviceID:          app.deviceInfo.GetID(),
		ClientID:          app.deviceInfo.GetClientID(),
		SkipTLSVerify:     app.flags.SkipTLSVerify,
		HandshakeTimeout:  config.HandshakeTimeout,
		HeartbeatInterval: config.HeartbeatInterval,
	}
	app.connectionManager = network.NewConnectionManager(connectionConfig)

	// 初始化输入管理器
	app.inputManager = interaction.NewInputManager()

	// 初始化唤醒词管理器（如果启用）
	if app.flags.EnableWakeWord {
		wakeWordConfig := interaction.WakeWordConfig{
			MaxRecordingDuration: config.MaxRecordingDuration,
			DebugEnabled:         app.flags.DebugEnabled,
		}
		app.wakeWordManager = interaction.NewWakeWordManager(wakeWordConfig)

		// 设置音效管理器
		if app.audioManager != nil {
			soundEffectsManager := app.audioManager.GetSoundEffectsManager()
			if soundEffectsManager != nil {
				app.wakeWordManager.SetSoundEffectsManager(soundEffectsManager)
				logrus.Debug("音效管理器已设置到唤醒词管理器")
			}
		}
	}

	// 创建客户端实例
	app.clientInstance = client.New(app.connectionManager.GetProtocol())
	app.clientInstance.SetDeviceID(app.deviceInfo.GetID())
	app.clientInstance.SetClientID(app.deviceInfo.GetClientID())
	if app.flags.Token != "" {
		app.clientInstance.SetToken(app.flags.Token)
	}

	// 设置VAD管理器的客户端实例
	if app.vadManager != nil {
		app.vadManager.SetClientInstance(app.clientInstance)
	}

	logrus.Info("应用程序初始化完成")
	return nil
}

// Run 运行应用程序
func (app *Application) Run() error {
	if app.flags.ActivateOnly {
		return app.activationManager.RunActivation()
	}

	// 启动输入管理器
	if err := app.inputManager.Start(); err != nil {
		return err
	}
	defer app.inputManager.Stop()

	// 显示帮助信息
	app.inputManager.SetupHelpDisplay(
		app.flags.EnableWakeWord,
		app.flags.AutoInteraction,
		app.flags.AutoInteractionSilenceThreshold,
	)

	// 设置回调
	if err := app.setupCallbacks(); err != nil {
		return err
	}

	// 初始化唤醒词检测（如果启用）
	if app.flags.EnableWakeWord {
		if err := app.setupWakeWord(); err != nil {
			logrus.Errorf("设置唤醒词检测失败: %v", err)
		} else {
			// 启动持续录音，用于唤醒词检测和语音输入
			if app.audioManager != nil && !app.audioManager.IsRecording() {
				logrus.Info("启动持续录音，用于唤醒词检测和语音输入...")
				if err := app.audioManager.StartRecording(); err != nil {
					logrus.Errorf("启动持续录音失败: %v", err)
				} else {
					logrus.Info("持续录音已启动，用于唤醒词检测和语音输入")
				}
			}
		}
	} else {
		app.setupNormalMode()
	}

	// 连接到服务器
	if err := app.connectionManager.Connect(); err != nil {
		return err
	}

	// 启动心跳
	app.connectionManager.StartHeartbeat()

	// 进入主循环
	return app.mainLoop()
}

// Stop 停止应用程序
func (app *Application) Stop() {
	if !app.isRunning {
		return
	}

	app.isRunning = false

	// 1. 首先断开网络连接，避免新的消息到达
	if app.connectionManager != nil {
		logrus.Debug("正在断开网络连接...")
		app.connectionManager.ForceDisconnect()
	}

	// 2. 停止唤醒词检测
	if app.wakeWordManager != nil {
		logrus.Debug("正在停止唤醒词检测...")
		app.wakeWordManager.Stop()
	}

	// 3. 停止输入管理器
	if app.inputManager != nil {
		logrus.Debug("正在停止输入管理器...")
		app.inputManager.Stop()
	}

	// 4. 关闭音频系统
	if app.audioManager != nil {
		logrus.Debug("正在关闭音频系统...")
		app.audioManager.Close()
	}

	// 5. 关闭客户端连接 (在退出时跳过复杂的CloseAudioChannel操作)
	// CloseAudioChannel可能会因为网络状态而阻塞，所以跳过这个步骤
	// 网络连接已经在步骤1中强制断开了
	if app.clientInstance != nil {
		logrus.Debug("客户端连接已在网络断开时清理")
	}

	logrus.Info("应用程序已停止")
}

// setupCallbacks 设置回调函数
func (app *Application) setupCallbacks() error {
	// 设置网络回调处理器
	callbackHandler := network.NewCallbackHandler(app.clientInstance)

	// 设置音频处理器
	callbackHandler.SetAudioHandler(func(data []byte) {
		app.playbackController.ProcessBinaryAudioData(data, app.flags.VerboseLogging)
	})

	// 设置TTS开始处理器
	callbackHandler.SetTTSStartHandler(func() {
		// 暂停VAD检测，避免在TTS播放期间误触发静音检测（检查是否已暂停，避免重复调用）
		if app.audioManager != nil && app.audioManager.VAD() != nil && !app.audioManager.VAD().IsPaused() {
			app.audioManager.PauseVAD()
		}
	})

	// 设置TTS停止处理器（用于自动交互）
	callbackHandler.SetTTSStopHandler(func() {
		// 使用统一的VAD管理器恢复检测
		if app.vadManager != nil {
			app.vadManager.Resume()
			logrus.Debug("🔊 统一VAD管理器已恢复，设置宽限期")
		}

		if app.flags.AutoInteraction {
			// Only trigger auto interaction if it's not already active to prevent loops
			if !app.autoInteractionActive {
				// 检查当前是否正在录音，如果是则让VAD超时机制处理
				if app.recordingController != nil && app.recordingController.IsRecording() {
					logrus.Info("🔄 TTS停止：检测到录音正在进行中，让VAD超时机制自然处理")
					// 确保自动交互模式设置正确
					if app.wakeWordManager != nil {
						app.wakeWordManager.SetAutoInteractionMode(true)
					}
					return
				}

				// 播放deng提示音表示进入连续交互模式
				if app.audioManager != nil {
					soundEffectsManager := app.audioManager.GetSoundEffectsManager()
					if soundEffectsManager != nil {
						soundEffectsManager.PlayDengSound()
					}
				}

				go app.triggerAutoInteraction()
			} else {
				logrus.Debug("TTS停止：自动交互已在进行中，跳过触发")
			}
		}
	})

	// 设置Hello消息处理器
	callbackHandler.SetHelloHandler(func(audioParams map[string]interface{}) {
		if sampleRate, ok := audioParams["sample_rate"].(int); ok {
			if channels, ok := audioParams["channels"].(int); ok {
				if frameDuration, ok := audioParams["frame_duration"].(int); ok {
					app.playbackController.ReinitializeDecoder(sampleRate, channels, frameDuration)
				}
			}
		}
	})

	// 设置STT消息处理器（语音识别结果）
	callbackHandler.SetSTTHandler(func(text string) {
		logrus.Infof("🎯 语音识别完成: '%s'，停止录音", text)

		// 当收到语音识别结果时，立即停止录音
		if app.recordingController != nil && app.recordingController.IsRecording() {
			logrus.Info("🔇 收到语音识别结果，立即停止录音")
			app.recordingController.StopRecording(app.clientInstance, app.flags.EnableWakeWord)
		}
	})

	// 设置连接管理器的回调
	handlers := &network.ConnectionHandlers{
		OnConnected: func() {
			logrus.Info("WebSocket连接成功!")
		},
		OnDisconnected: func(err error) {
			if err != nil {
				logrus.Errorf("WebSocket断开连接: %v", err)
			} else {
				logrus.Info("WebSocket正常断开连接")
			}
		},
		OnJSONMessage: func(data []byte) {
			callbackHandler.HandleJSONMessage(data, app.flags.VerboseLogging)
		},
		OnBinaryMessage: func(data []byte) {
			callbackHandler.HandleBinaryMessage(data, app.flags.VerboseLogging)
		},
	}

	app.connectionManager.SetHandlers(handlers)
	app.connectionManager.SetClient(app.clientInstance)

	// 设置客户端回调
	callbackHandler.SetupClientCallbacks()

	return nil
}

// setupWakeWord 设置唤醒词检测
func (app *Application) setupWakeWord() error {
	if app.wakeWordManager == nil {
		return errors.New("wake word manager not initialized")
	}

	// 设置音频数据回调（用于发送到服务器）
	app.recordingController.SetupAudioCallback("server_sender_wake_word", true)

	// 设置音频PCM回调（用于唤醒词检测和VAD处理）
	app.recordingController.SetupPCMCallback("wake_word_detector", func(data []int16) {
		// 处理唤醒词检测
		app.wakeWordManager.ProcessAudioData(data)

		// 处理VAD逻辑
		// If we're in auto interaction mode, handle VAD differently than in normal wake word mode
		if app.wakeWordManager.IsAutoInteractionMode() {
			// In auto interaction mode, we handle VAD silence detection separately in monitorVADSilence
			// Just process the audio for the VAD system
			if app.audioManager.VAD() != nil {
				app.audioManager.ProcessVADAudio(data)
			}
		}
	})

	// 初始化唤醒词检测器
	err := app.wakeWordManager.Initialize(
		app.clientInstance,
		func(keyword string) {
			app.recordingController.StartRecording(app.clientInstance, true)
		},
	)

	if err != nil {
		return err
	}

	// 启动唤醒词检测
	return app.wakeWordManager.Start()
}

// setupNormalMode 设置正常模式
func (app *Application) setupNormalMode() {
	// 设置音频回调
	app.recordingController.SetupAudioCallback("normal_mode_sender", false)

	// 设置PCM回调用于VAD
	app.recordingController.SetupPCMCallback("normal_mode_vad", func(data []int16) {
		if app.audioManager.VAD() != nil {
			app.audioManager.ProcessVADAudio(data)
		}
	})
}

// triggerAutoInteraction 触发自动交互模式
func (app *Application) triggerAutoInteraction() {
	// 设置自动交互激活标志，防止循环触发
	app.autoInteractionActive = true
	defer func() {
		// 确保在函数退出时重置标志
		app.autoInteractionActive = false
		// 注意：不要重置自动交互模式，让下一个TTS周期自然处理
	}()

	logrus.Info("🚀 自动交互模式触发开始...")

	// 检查客户端是否已连接到服务器
	if !app.connectionManager.IsConnected() {
		logrus.Warn("自动交互模式：客户端未连接到服务器，跳过自动录音")
		return
	}

	// 检查当前状态
	currentState := app.clientInstance.GetState()
	logrus.Infof("🔍 自动交互模式：当前客户端状态: %s", currentState)

	// 如果正在播放AI回复，等待状态更新
	if currentState == client.StateSpeaking {
		logrus.Info("⏸️ 自动交互模式：检测到speaking状态，等待状态更新...")
		for i := 0; i < 10; i++ {
			time.Sleep(100 * time.Millisecond)
			currentState = app.clientInstance.GetState()
			if currentState != client.StateSpeaking {
				logrus.Infof("✅ 自动交互模式：状态已更新为: %s", currentState)
				break
			}
		}

		// 如果等待后仍然是speaking状态，手动设置为idle
		if currentState == client.StateSpeaking {
			logrus.Info("⚠️ 自动交互模式：状态未自动更新，手动设置为idle")
			app.clientInstance.SetState(client.StateIdle)
			currentState = client.StateIdle
		}
	}

	// 延迟一小段时间，确保TTS完全播放完毕
	logrus.Debug("⏳️ 自动交互模式：等待TTS完全播放完毕...")
	time.Sleep(500 * time.Millisecond)

	// 关键修复：无论当前状态如何，都要先停止当前录音，然后重新开始
	// 这确保了录音周期的完整性
	if app.recordingController.IsRecording() {
		logrus.Info("🔄 自动交互模式：停止当前录音周期...")
		app.recordingController.StopRecording(app.clientInstance, app.flags.EnableWakeWord)
	}

	// 等待一小段时间确保停止完成
	time.Sleep(200 * time.Millisecond)

	// 手动设置客户端状态为idle，这样SendStartListening才能成功
	logrus.Debug("🔄 自动交互模式：手动设置客户端状态为idle")
	app.clientInstance.SetState(client.StateIdle)

	// 发送开始监听命令到服务器
	logrus.Info("🎤 自动交互模式：发送开始监听命令到服务器...")
	if err := app.clientInstance.SendStartListening(client.ListenModeAuto); err != nil {
		logrus.Errorf("❌ 自动交互模式：发送开始监听命令失败: %v", err)
		return
	}

	// 等待状态变更
	logrus.Info("⏳️ 自动交互模式：等待状态变更为监听状态...")
	stateChangeTimeout := time.After(3 * time.Second)
	for {
		currentState = app.clientInstance.GetState()
		if currentState == client.StateListening {
			logrus.Info("✅ 自动交互模式：状态已变更为监听状态")
			break
		}
		select {
		case <-stateChangeTimeout:
			logrus.Warn("⚠️ 自动交互模式：等待状态变更超时")
			return
		case <-time.After(100 * time.Millisecond):
			// 继续等待
		}
	}

	// 如果启用了VAD和自动交互模式，使用统一的VAD管理器
	if app.flags.EnableVAD && app.flags.AutoInteraction && app.vadManager != nil {
		logrus.Info("🎯 自动交互模式：使用统一VAD管理器进行静音检测...")

		// 设置VAD静音超时回调
		app.vadManager.SetSilenceTimeoutCallback(func() {
			logrus.Info("🔇 统一VAD管理器：检测到静音超时，自动停止录音")
			app.recordingController.StopRecording(app.clientInstance, app.flags.EnableWakeWord)
			// 强制设置客户端状态为idle，确保状态同步
			app.clientInstance.SetState(client.StateIdle)
		})

		// 启动VAD管理器
		if err := app.vadManager.Start(); err != nil {
			logrus.Errorf("启动VAD管理器失败: %v", err)
		}
	}

	// If wake word manager exists, set auto interaction mode
	if app.wakeWordManager != nil {
		app.wakeWordManager.SetAutoInteractionMode(true)
	}

	// 开始录音（在设置VAD回调之后）
	logrus.Info("🎤 自动交互模式：开始新的录音周期...")
	app.recordingController.StartRecording(app.clientInstance, app.flags.EnableWakeWord)

	}


// mainLoop 主循环
func (app *Application) mainLoop() error {
	app.isRunning = true

	for app.isRunning {
		select {
		case cmd := <-app.inputManager.GetCommandChannel():
			if !app.isRunning { // 检查是否在处理命令过程中被停止
				return nil
			}
			logrus.Debugf("主循环收到命令: %s", cmd)
			if cmd == "quit" || cmd == "q" {
				logrus.Info("收到退出命令，准备退出程序...")
				app.Stop()
				return nil
			} else {
				logrus.Warnf("不支持的命令: %s", cmd)
			}

		case key := <-app.inputManager.GetKeyPressChannel():
			if !app.isRunning { // 检查是否在处理按键过程中被停止
				return nil
			}
			logrus.Debugf("主循环收到按键事件: %s", key)
			app.handleKeyPress(key)
		}
	}

	return nil
}

// handleKeyPress 处理按键事件
func (app *Application) handleKeyPress(key string) {
	// 如果应用已停止，不再处理按键
	if !app.isRunning {
		return
	}

	isRecording := app.recordingController.IsRecording()

	if key == "F2_PRESSED" && !isRecording {
		// 检查客户端是否已连接到服务器
		if !app.connectionManager.IsConnected() {
			logrus.Error("客户端未连接到服务器，无法开始录音")
			fmt.Println("⚠️ 未连接到服务器，请先使用/connect命令连接")
			return
		}

		logrus.Info("开始录音...")

		// 检查客户端当前状态
		currentState := app.clientInstance.GetState()
		if currentState == client.StateSpeaking {
			logrus.Info("正在中断AI回复以开始录音...")
			app.clientInstance.SendAbortSpeaking("start_recording")
			app.playbackController.StopAudioPlayback()
		}

		// 开始录音
		app.recordingController.StartRecording(app.clientInstance, app.flags.EnableWakeWord)

	} else if key == "F3_PRESSED" {
		// 测试deng声音功能
		logrus.Info("测试deng提示音...")
		if app.audioManager != nil {
			soundEffectsManager := app.audioManager.GetSoundEffectsManager()
			if soundEffectsManager != nil {
				soundEffectsManager.PlayDengSound()
				fmt.Println("🔔 正在播放deng测试音...")
			} else {
				fmt.Println("❌ 音效管理器未初始化")
			}
		} else {
			fmt.Println("❌ 音频管理器未初始化")
		}

	} else if key == "F2_RELEASED" {
		// 检查客户端当前状态
		currentState := app.clientInstance.GetState()
		if currentState == client.StateSpeaking {
			logrus.Info("正在中断AI回复...")
			if err := app.clientInstance.SendAbortSpeaking("stop_speaking"); err != nil {
				logrus.Errorf("发送停止讲话命令失败: %v", err)
			}
			app.playbackController.StopAudioPlayback()
		}

		// 停止录音
		if isRecording {
			logrus.Info("停止录音...")
			app.recordingController.StopRecording(app.clientInstance, app.flags.EnableWakeWord)
		}
	}
}

// IsRunning 检查应用程序是否正在运行
func (app *Application) IsRunning() bool {
	return app.isRunning
}
