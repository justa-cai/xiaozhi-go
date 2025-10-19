package main

import (
	"crypto/md5"
	"crypto/rand"
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/justa-cai/xiaozhi-go/internal/audio"
	"github.com/justa-cai/xiaozhi-go/internal/client"
	"github.com/justa-cai/xiaozhi-go/internal/ota"
	"github.com/justa-cai/xiaozhi-go/internal/protocol"
	"github.com/justa-cai/xiaozhi-go/internal/wakeword"
	"github.com/sirupsen/logrus"
)

// 常量
const (
	StateIdle      = "idle"
	StateListening = "listening"
	StateSpeaking  = "speaking"
)

var (
	// 命令行参数
	serverURL     string
	deviceID      string
	token         string
	boardType     string
	appVersion    string
	activateOnly  bool
	logLevel      string
	skipTLSVerify bool
	httpProxy     string
	// 添加调试标志
	debugEnabled bool
	// 添加详细日志标志
	verboseLogging bool
	// 添加唤醒词检测标志
	enableWakeWord bool
	// 添加自动交互模式标志
	autoInteraction bool
	// 添加VAD标志
	enableVAD bool
	// 添加静音超时时间（毫秒）
	silenceTimeoutMs int
	// 添加自动交互模式静音时间阈值（秒）
	autoInteractionSilenceThreshold int
)

// 全局音频管理器
var (
	audioManager *audio.AudioManagerNew
	audioPlayer  *audio.AudioPlayerNew
)

// 全局唤醒词检测器
var (
	wakeWordDetector *wakeword.WakeWordDetector
	wakeWordTimer    *time.Timer
	// 全局VAD控制变量，用于自动交互模式
	vadWakeWordDetected bool = false
	vadSilenceStartTime time.Time
)

// 全局录音控制标志
var (
	sendToServer bool = false
)

// 定义一个全局变量，用于追踪是否已恢复终端设置
var terminalRestored bool = false
var terminalMutex sync.Mutex

// 全局音频数据通道
var audioChan chan []byte

var audioInited = false

func init() {
	// 解析命令行参数
	flag.StringVar(&serverURL, "server", protocol.DefaultWebSocketURL, "WebSocket服务器地址")
	flag.StringVar(&deviceID, "device-id", "", "设备ID (MAC地址)")
	flag.StringVar(&token, "token", "test-token", "API访问令牌")
	flag.StringVar(&boardType, "board", "generic", "设备板型号")
	flag.StringVar(&appVersion, "version", "1.0.0", "应用版本号")
	flag.BoolVar(&activateOnly, "activate-only", false, "只执行激活流程")
	flag.StringVar(&logLevel, "log-level", "info", "日志级别 (debug, info, warn, error, fatal, panic)")
	flag.BoolVar(&skipTLSVerify, "skip-tls-verify", true, "跳过TLS证书验证")
	flag.StringVar(&httpProxy, "http-proxy", "", "HTTP代理地址，例如: http://127.0.0.1:8080")
	// 添加调试标志
	flag.BoolVar(&debugEnabled, "debug", false, "启用高级调试功能")
	// 添加详细日志标志
	flag.BoolVar(&verboseLogging, "verbose", false, "启用详细日志")
	// 添加唤醒词检测标志
	flag.BoolVar(&enableWakeWord, "wakeword", false, "启用唤醒词检测功能")
	// 添加自动交互模式标志
	flag.BoolVar(&autoInteraction, "auto-interaction", true, "启用自动交互模式（TTS播放结束后自动开始录音）")
	// 添加VAD标志
	flag.BoolVar(&enableVAD, "vad", true, "启用高级语音活动检测(VAD)功能，提供更准确的人声检测")
	// 添加静音超时时间（毫秒）
	flag.IntVar(&silenceTimeoutMs, "silence-timeout", 800, "静音超时时间（毫秒），超过此时间无语音则自动停止录音")
	// 添加自动交互模式静音时间阈值
	flag.IntVar(&autoInteractionSilenceThreshold, "auto-silence-threshold", 5, "自动交互模式静音时间阈值（秒），超过此时间无语音则自动停止录音")

	// 配置日志
	logrus.SetFormatter(&logrus.TextFormatter{
		FullTimestamp:   true,
		TimestampFormat: "2006-01-02 15:04:05",
	})

	// 添加一个日志钩子，以便跟踪WebSocket连接过程
	logrus.AddHook(&WebSocketLogHook{})
}

// WebSocketLogHook 是一个简单的日志钩子，用于跟踪WebSocket连接
type WebSocketLogHook struct{}

// Levels 指定此钩子将处理的日志级别
func (hook *WebSocketLogHook) Levels() []logrus.Level {
	return []logrus.Level{
		logrus.DebugLevel,
		logrus.InfoLevel,
		logrus.WarnLevel,
		logrus.ErrorLevel,
	}
}

// Fire 处理日志条目
func (hook *WebSocketLogHook) Fire(entry *logrus.Entry) error {
	// 只保留关键连接信息的详细日志，避免重复输出普通日志
	msg := entry.Message
	if (strings.Contains(msg, "WebSocket连接成功") ||
		strings.Contains(msg, "连接失败") ||
		strings.Contains(msg, "hello消息") ||
		strings.Contains(msg, "断开连接")) &&
		entry.Level <= logrus.InfoLevel {
		// 将WebSocket连接关键消息保存到日志文件或特殊格式输出
		fmt.Printf("[WS-CONNECTION] %s: %s\n",
			entry.Time.Format("15:04:05.000"),
			entry.Message)
	}
	return nil
}

// safeExit 安全退出程序，确保恢复终端设置
func safeExit(code int) {
	terminalMutex.Lock()
	defer terminalMutex.Unlock()

	if !terminalRestored {
		// 恢复终端设置
		if err := exec.Command("stty", "-F", "/dev/tty", "echo").Run(); err != nil {
			logrus.Errorf("退出时恢复终端回显失败: %v", err)
		}
		if err := exec.Command("stty", "-F", "/dev/tty", "-cbreak").Run(); err != nil {
			logrus.Errorf("退出时恢复终端规范模式失败: %v", err)
		}
		terminalRestored = true
		logrus.Debug("退出前已恢复终端设置")
	}

	os.Exit(code)
}

// cleanupAndExit 清理资源并安全退出
func cleanupAndExit(c *client.Client, code int) {
	// 直接强制退出，不等待资源清理
	// 设置一个非常短的超时时间
	forcedExit := make(chan struct{})
	go func() {
		select {
		case <-forcedExit:
			return
		case <-time.After(1 * time.Second):
			logrus.Warn("强制结束进程")
			safeExit(1)
		}
	}()

	// 快速清理核心资源
	logrus.Debug("开始快速清理资源...")

	// 使用goroutine并行处理所有清理工作
	var wg sync.WaitGroup

	// 关闭唤醒词检测器
	wg.Add(1)
	go func() {
		defer wg.Done()
		if wakeWordDetector != nil {
			logrus.Debug("正在停止唤醒词检测器...")
			if err := wakeWordDetector.Stop(); err != nil {
				logrus.Errorf("停止唤醒词检测器失败: %v", err)
			}
		}
		// 停止唤醒词定时器
		if wakeWordTimer != nil {
			wakeWordTimer.Stop()
			wakeWordTimer = nil
			logrus.Debug("已停止唤醒词定时器")
		}
	}()

	// 关闭客户端连接 - 最优先处理
	if c != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()

			cleanDone := make(chan struct{})
			go func() {
				logrus.Debug("正在关闭音频通道...")
				// 直接强制关闭连接，不调用客户端的方法
				if proto := c.GetProtocol(); proto != nil {
					if wp, ok := proto.(*protocol.WebsocketProtocol); ok {
						wp.ForceDisconnect()
					} else {
						// 普通关闭
						c.CloseAudioChannel()
					}
				}
				close(cleanDone)
			}()

			// 最多等待200ms
			select {
			case <-cleanDone:
				logrus.Debug("音频通道已关闭")
			case <-time.After(200 * time.Millisecond):
				logrus.Warn("关闭音频通道超时")
			}
		}()
	}

	// 等待所有清理工作完成或超时
	waitChan := make(chan struct{})
	go func() {
		wg.Wait()
		close(waitChan)
	}()

	// 最多等待500ms
	select {
	case <-waitChan:
		// 所有工作完成
	case <-time.After(500 * time.Millisecond):
		logrus.Warn("资源清理超时")
	}

	// 关闭强制退出
	close(forcedExit)

	// 立即退出
	logrus.Info("正在退出程序")
	safeExit(code)
}

// analyzeConnectionError 分析连接错误
func analyzeConnectionError(err error) {
	logrus.Error("连接错误详细分析:")

	if os.IsTimeout(err) {
		logrus.Error("- 错误类型: 连接超时")
		logrus.Error("- 可能原因: 网络延迟高、服务器无响应或防火墙阻止")
		logrus.Error("- 建议解决方案: 检查网络连接、确认服务器地址正确、检查防火墙设置")
	} else if strings.Contains(err.Error(), "certificate") {
		logrus.Error("- 错误类型: 证书验证错误")
		logrus.Error("- 可能原因: 自签名证书或证书无效")
		logrus.Error("- 建议解决方案: 使用 --skip-tls-verify 选项跳过证书验证")
	} else if strings.Contains(err.Error(), "dial") {
		logrus.Error("- 错误类型: 网络连接错误")
		logrus.Error("- 可能原因: 网络不可达、端口关闭或主机不存在")
		logrus.Error("- 建议解决方案: 确认服务器地址和端口正确、检查网络配置")
	} else if strings.Contains(err.Error(), "proxy") {
		logrus.Error("- 错误类型: 代理连接错误")
		logrus.Error("- 可能原因: 代理配置错误或代理服务不可用")
		logrus.Error("- 建议解决方案: 检查代理配置或暂时禁用代理")
	} else {
		logrus.Error("- 错误类型: 未知错误")
		logrus.Error("- 错误详情:", err.Error())
		logrus.Error("- 建议解决方案: 检查网络环境和服务器状态")
	}
}

func main() {
	flag.Parse()

	// 根据命令行参数设置日志级别
	switch strings.ToLower(logLevel) {
	case "debug":
		logrus.SetLevel(logrus.DebugLevel)
	case "info":
		logrus.SetLevel(logrus.InfoLevel)
	case "warn":
		logrus.SetLevel(logrus.WarnLevel)
	case "error":
		logrus.SetLevel(logrus.ErrorLevel)
	case "fatal":
		logrus.SetLevel(logrus.FatalLevel)
	case "panic":
		logrus.SetLevel(logrus.PanicLevel)
	default:
		logrus.Warnf("未知的日志级别: %s，使用默认级别 debug", logLevel)
		logrus.SetLevel(logrus.DebugLevel)
	}

	// 在程序退出时确保恢复终端设置
	defer func() {
		exec.Command("stty", "-F", "/dev/tty", "echo").Run()
		exec.Command("stty", "-F", "/dev/tty", "-cbreak").Run()
		logrus.Debug("已恢复终端设置")
	}()

	logrus.Info("正在启动小智客户端...")

	// 显示自动交互模式状态
	if autoInteraction {
		logrus.Infof("🔄 自动交互模式已启用，静音阈值：%d秒", autoInteractionSilenceThreshold)
	} else {
		logrus.Info("📱 自动交互模式已禁用")
	}

	// 显示VAD功能状态
	if enableVAD {
		logrus.Info("🎙️ 高级VAD语音检测已启用，提供更准确的人声识别和静音检测")
	} else {
		logrus.Info("🔊 使用简单能量阈值检测")
	}

	// 获取设备ID
	if deviceID == "" {
		var err error
		deviceID, err = getMACAddress()
		if err != nil {
			logrus.Warnf("无法获取MAC地址: %v", err)
			deviceID = fmt.Sprintf("device-%d", time.Now().Unix())
			logrus.Infof("生成临时设备ID: %s", deviceID)
		}
	}
	logrus.Infof("使用设备ID: %s", deviceID)

	// 如果只执行激活流程
	if activateOnly {
		runActivation()
		return
	}

	// 如果设备未激活，则返回
	if !isDeviceActivated() {
		logrus.Error("设备未激活，请先激活设备")
		return
	}

	// 初始化音频系统
	initAudio()
	defer cleanupAudio()

	// 创建WebSocket协议实例
	proto := protocol.NewWebsocketProtocol()

	// 设置跳过TLS证书验证
	proto.SetSkipTLSVerify(skipTLSVerify)
	if skipTLSVerify {
		logrus.Info("已设置跳过TLS证书验证")
	} else {
		logrus.Info("将验证TLS证书")
	}

	// 创建客户端
	c := client.New(proto)
	c.SetDeviceID(deviceID)

	// 使用基于设备ID生成的UUID作为客户端ID
	clientID := generateUUID(deviceID)
	c.SetClientID(clientID)
	logrus.Infof("使用客户端ID: %s", clientID)

	if token != "" {
		c.SetToken(token)
	}

	// 捕获中断信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// 确保信号处理不会被阻塞
	go func() {
		sig := <-sigChan
		logrus.Infof("接收到信号: %v, 立即退出...", sig)

		// 使用cleanupAndExit功能进行资源清理和安全退出
		cleanupAndExit(c, 0)
	}()

	// 设置回调
	setupCallbacks(c)

	// 设置连接回调
	proto.SetOnConnected(func() {
		logrus.Info("✅ WebSocket连接成功!")

		// 发送hello消息
		helloMsg := map[string]interface{}{
			"type":      "hello",
			"version":   1,
			"transport": "websocket",
			"audio_params": map[string]interface{}{
				"format":         "opus",
				"sample_rate":    16000,
				"channels":       1,
				"frame_duration": 60,
			},
		}

		if err := proto.SendJSON(helloMsg); err != nil {
			logrus.Errorf("❌ 发送hello消息失败: %v", err)
		} else {
			logrus.Info("✅ hello消息发送成功")
		}
	})

	proto.SetOnDisconnected(func(err error) {
		if err != nil {
			logrus.Errorf("❌ WebSocket断开连接: %v", err)

			// 延迟1秒后尝试重连
			go func() {
				logrus.Info("准备在1秒后尝试重新连接...")
				time.Sleep(1 * time.Second)

				logrus.Info("正在尝试重新连接...")
				// 设置请求头
				proto.SetHeader("Authorization", fmt.Sprintf("Bearer %s", token))
				proto.SetHeader("Protocol-Version", "1")
				proto.SetHeader("Device-Id", deviceID)
				proto.SetHeader("Client-Id", generateUUID(deviceID))

				// 连接
				if err := proto.Connect(serverURL); err != nil {
					logrus.Errorf("重新连接失败: %v", err)
					analyzeConnectionError(err)
				} else {
					logrus.Info("✅ 重新连接成功")
				}
			}()
		} else {
			logrus.Info("WebSocket正常断开连接")
		}
	})

	// 设置JSON消息回调
	proto.SetOnJSONMessage(func(data []byte) {
		// 尝试解析JSON格式以便美观打印
		var jsonData interface{}
		if err := json.Unmarshal(data, &jsonData); err == nil {
			if verboseLogging {
				jsonBytes, _ := json.MarshalIndent(jsonData, "", "  ")
				logrus.Infof("📥 接收到JSON数据: \n%s", string(jsonBytes))
			} else {
				// 简化输出，只显示消息类型
				if typeMap, ok := jsonData.(map[string]interface{}); ok {
					if msgType, exists := typeMap["type"]; exists {
						jsonBytes, _ := json.MarshalIndent(jsonData, "", "  ")
						logrus.Infof("📥 接收到消息类型: %v %s", msgType, string(jsonBytes))

						// 处理服务器的hello消息
						if msgType == "hello" {
							// 检查是否包含音频参数
							if audioParams, ok := typeMap["audio_params"].(map[string]interface{}); ok {
								logrus.Info("收到服务器hello消息，包含音频参数")
								// 提取音频参数
								sampleRate, _ := audioParams["sample_rate"].(float64)
								channels, _ := audioParams["channels"].(float64)
								frameDuration, _ := audioParams["frame_duration"].(float64)
								format, _ := audioParams["format"].(string)

								// 验证音频参数有效性
								if sampleRate > 0 && channels > 0 && frameDuration > 0 && format != "" {
									logrus.Infof("重新初始化解码器: format=%s, sample_rate=%v, channels=%v, frame_duration=%v",
										format, sampleRate, channels, frameDuration)
									// 调用重新初始化解码器的函数
									reinitializeOpusDecoder(int(sampleRate), int(channels), int(frameDuration))
								}
							}
						}

						// 处理TTS停止消息，触发自动交互模式
						if msgType == "tts" {
							if state, exists := typeMap["state"].(string); exists && state == "stop" {
								logrus.Info("🔄 检测到TTS播放结束，更新客户端状态...")

								// 手动更新客户端状态从speaking到idle
								currentState := c.GetState()
								logrus.Infof("📝 TTS停止前客户端状态: %s", currentState)
								if currentState == client.StateSpeaking {
									c.SetState(client.StateIdle)
									logrus.Infof("✅ 已将客户端状态从 %s 更新为 %s", client.StateSpeaking, client.StateIdle)
								}

								// 如果启用了自动交互模式，触发自动交互
								if autoInteraction {
									logrus.Info("🔄 检测到TTS播放结束，自动交互模式启动...")
									// 异步执行自动交互模式，避免阻塞WebSocket消息处理
									go func() {
										time.Sleep(100 * time.Millisecond) // 短暂延迟确保状态更新
										triggerAutoInteraction(c)
									}()
								}
							}
						}
					} else {
						logrus.Info("📥 接收到JSON数据")
					}
				}
			}
		} else {
			logrus.Infof("📥 接收到文本数据: %s", string(data))
		}
	})

	// 设置二进制消息回调
	proto.SetOnBinaryMessage(func(data []byte) {
		if verboseLogging {
			logrus.Infof("📥 接收到二进制数据: %d字节", len(data))
		}

		// 处理Opus编码的音频数据
		if audioManager != nil && audioManager.Player() != nil {
			// 检查音频播放器状态
			if !audioManager.Player().IsPlaying() {
				// 播放器未运行，可能是因为刚初始化或之前有错误
				logrus.Debug("音频播放器未运行，尝试启动...")
				if err := audioManager.Player().Start(); err != nil {
					logrus.Errorf("启动音频播放器失败: %v", err)
				}
			}

			// 如果播放器在哑模式下运行，记录一下
			if audioManager.Player().IsDummyMode() && verboseLogging {
				logrus.Debug("音频播放器在哑模式下运行，可能无法实际播放音频")
			}

			c.SetState(client.StateSpeaking)
			// 将Opus编码的音频数据添加到播放队列
			audioManager.Player().QueueAudio(data)

			if verboseLogging {
				logrus.Debugf("已将%d字节Opus编码音频数据添加到播放队列", len(data))
			}
		} else {
			logrus.Warn("音频播放器未初始化，无法播放收到的音频数据")
			// 不再尝试 new audioPlayer，直接报错
		}
	})

	// 显示操作说明
	if enableWakeWord {
		fmt.Println("唤醒词检测模式已启用:")
		if autoInteraction {
			fmt.Printf("  🔄 自动交互模式已启用（TTS播放结束后自动开始录音，静音阈值：%d秒）\n", autoInteractionSilenceThreshold)
		}
		fmt.Println("  说出 '你好小智' 或 '小智同学' 来激活助手")
		fmt.Println("  f - 开始录音")
		fmt.Println("  s - 停止录音")
		fmt.Println("  q - 退出程序")
	} else {
		if autoInteraction {
			fmt.Printf("🔄 自动交互模式已启用（TTS播放结束后自动开始录音，静音阈值：%d秒）\n", autoInteractionSilenceThreshold)
		}
		fmt.Println("按键操作:")
		fmt.Println("  f - 开始录音")
		fmt.Println("  s - 停止录音")
		fmt.Println("  q - 退出程序")
	}

	// 启动按键监听
	keyPressCh := make(chan string)
	commandCh := make(chan string)
	go readInput(keyPressCh, commandCh)

	// 记录录音状态
	isRecording := false

	// 连接服务器
	logrus.Info("准备连接到服务器...")

	// 添加请求头
	proto.SetHeader("Authorization", fmt.Sprintf("Bearer %s", token))
	proto.SetHeader("Protocol-Version", "1")
	proto.SetHeader("Device-Id", deviceID)
	proto.SetHeader("Client-Id", generateUUID(deviceID))

	// 设置握手超时
	proto.SetHandshakeTimeout(15 * time.Second)

	// 初始化唤醒词检测器（如果启用）
	if enableWakeWord {
		logrus.Info("正在初始化唤醒词检测器...")
		var err error

		// 最大录音时长10秒
		const maxRecordingDuration = 10 * time.Second

		wakeWordDetector, err = wakeword.NewWakeWordDetector(
			func(keyword string) {
				// 唤醒词检测回调函数
				logrus.Infof("唤醒词 '%s' 检测到！激活助手...", keyword)

				// 检查客户端是否已连接到服务器
				if !c.GetProtocol().IsConnected() {
					logrus.Error("客户端未连接到服务器，无法开始录音")
					return
				}

				// 检查当前状态
				currentState := c.GetState()
				if currentState == client.StateListening {
					// 如果已经在监听状态，说明是再次唤醒
					logrus.Info("客户端已在监听状态，检测到唤醒词...")
					vadWakeWordDetected = true
				} else {
					// 如果不在监听状态，开始监听（等同于按F键）
					logrus.Info("进入监听模式（等同于按F键）...")

					// 发送开始录音命令到服务器
					if err := c.SendStartListening(client.ListenModeManual); err != nil {
						logrus.Errorf("发送开始监听命令失败: %v", err)
						return
					}
					logrus.Info("已向服务器发送开始监听命令，准备接收语音输入...")

					// 开始录音 - 在唤醒词检测模式下，我们只需确保音频数据被发送到服务器
					// 音频管理器应该已经在运行，我们只需要激活音频数据发送
					logrus.Infof("唤醒词检测回调：开始录音，当前客户端状态: %s", c.GetState())
					startRecording(c)

					// 设置全局VAD标志
					vadWakeWordDetected = true
					// 重置静音开始时间
					vadSilenceStartTime = time.Time{} // 零值，表示还没有静音

					// 启动VAD监控goroutine（如果启用高级VAD）
					if enableVAD {
						go monitorVADSilence(c)
					}

					// 设置10秒超时定时器
					if wakeWordTimer != nil {
						wakeWordTimer.Stop()
					}
					wakeWordTimer = time.AfterFunc(maxRecordingDuration, func() {
						if vadWakeWordDetected && c.GetState() == client.StateListening {
							logrus.Info("录音达到最大时长10秒，自动停止录音（等同于按S键）...")
							stopRecording(c)
							vadWakeWordDetected = false
						}
					})
					logrus.Infof("已设置%d秒自动停止定时器", maxRecordingDuration/time.Second)
				}
			},
			func() {
				// End of speech callback - called when silence is detected after wake word
				// 注意：这个回调主要用作备用，实际的VAD检测现在由集成的VAD系统处理
				logrus.Debug("唤醒词检测器VAD检测到静音...")
				logrus.Debugf("唤醒词VAD回调触发时的客户端状态: %s, vadWakeWordDetected: %v", c.GetState(), vadWakeWordDetected)

				// 只有在没有启用集成VAD系统时才使用唤醒词检测器的VAD
				if vadWakeWordDetected && !enableVAD {
					if c.GetState() == client.StateListening {
						// 如果是第一次检测到静音，记录静音开始时间
						if vadSilenceStartTime.IsZero() {
							vadSilenceStartTime = time.Now()
							logrus.Infof("🎯 唤醒词VAD模式：检测到静音，开始记录静音时间，静音开始于: %v", vadSilenceStartTime.Format("15:04:05.000"))
							return // 第一次检测到静音时不停止，等待持续静音
						}

						// 计算持续静音的时间
						silenceDuration := time.Since(vadSilenceStartTime)

						// 根据录音模式设置不同的静音超时时间
						var silenceThreshold time.Duration
						var modeDesc string

						// 检查是否是自动交互模式
						if autoInteraction && c.GetState() == client.StateListening && sendToServer {
							silenceThreshold = time.Duration(autoInteractionSilenceThreshold) * time.Second
							modeDesc = "自动交互模式"
						} else {
							silenceThreshold = 3 * time.Second
							modeDesc = "唤醒词模式"
						}

						// 每秒输出一次静音时长日志，避免刷屏
						if int(silenceDuration.Seconds()) != int((silenceDuration - time.Second).Seconds()) {
							logrus.Infof("🎯 %s（唤醒词VAD）：持续静音时间: %.1f秒，阈值: %.1f秒", modeDesc, silenceDuration.Seconds(), float64(silenceThreshold)/float64(time.Second))
						}

						if silenceDuration >= silenceThreshold {
							logrus.Infof("✅ 连续%.1f秒检测不到声音，自动停止录音（%s，等同于按S键）...",
								float64(silenceThreshold)/float64(time.Second), modeDesc)
							// 停止录音（等同于按S键）
							stopRecording(c)
							// 重置标志和定时器
							vadWakeWordDetected = false
							vadSilenceStartTime = time.Time{} // 重置静音时间
							if wakeWordTimer != nil {
								wakeWordTimer.Stop()
								wakeWordTimer = nil
							}
						}
					} else {
						logrus.Debugf("🎯 唤醒词VAD模式：客户端状态为 %s，不处理静音", c.GetState())
					}
				} else {
					logrus.Debug("集成VAD已启用，忽略唤醒词检测器的VAD回调")
				}
			})
		if err != nil {
			logrus.Errorf("初始化唤醒词检测器失败: %v", err)
		} else {
			if err := wakeWordDetector.Start(); err != nil {
				logrus.Errorf("启动唤醒词检测器失败: %v", err)
			} else {
				logrus.Info("唤醒词检测器已启动")

				// 设置静音超时时间为1秒，使VAD更频繁地调用静音检测回调
				// 这样我们可以在回调中自己控制3秒的静音检测逻辑
				silenceTimeout := 1 * time.Second
				wakeWordDetector.SetSilenceTimeout(silenceTimeout)
				logrus.Infof("VAD静音超时设置为: %v（用于更频繁的静音检测）", silenceTimeout)

				// 设置音频数据回调 to send data to server when recording is active
				// We'll set it initially but control sending via a flag
				if audioManager != nil {
					// Add audio data callback to conditionally send to server
					audioManager.AddAudioDataCallback("server_sender", func(data []byte) {
						// logrus.Debugf("音频数据回调被调用，大小: %d 字节，sendToServer: %v, audioChan: %v", len(data), sendToServer, audioChan != nil)
						if sendToServer && audioChan != nil {
							// logrus.Debugf("接收到音频数据，大小: %d 字节，发送到服务器 (WakeWord Mode)", len(data))

							// 发送到通道，不阻塞
							select {
							case audioChan <- data:
								// logrus.Debugf("音频数据已发送到通道，大小: %d 字节 (WakeWord Mode)", len(data))
							default:
								// 通道已满，丢弃此数据包
								logrus.Warn("音频数据通道已满，丢弃数据包 (WakeWord Mode)")
							}
						}
						//  else {
						// When not sending to server, just log for debugging
						// logrus.Debugf("音频数据已接收但未发送到服务器（当前非录音状态或通道未初始化），sendToServer: %v, audioChan: %v", sendToServer, audioChan != nil)
						// }
					})

					// Add PCM callback for wake word detection and VAD processing
					audioManager.AddPCMDataCallback("wake_word_detector", func(data []int16, size int) {
						// logrus.Debugf("PCM音频数据回调被调用，大小: %d", size)
						// 复制数据以避免竞争条件
						dataCopy := make([]int16, size)
						copy(dataCopy, data[:size])

						// 将音频数据传递给唤醒词检测器 (always, even when recording)
						if wakeWordDetector != nil && wakeWordDetector.IsRunning() {
							wakeWordDetector.ProcessAudioData(dataCopy)
						}

						// 使用集成的VAD检测器处理音频数据
						if audioManager.VAD() != nil {
							if err := audioManager.ProcessVADAudio(dataCopy); err != nil && debugEnabled {
								logrus.Debugf("VAD处理音频数据失败: %v", err)
							}
						}

						// VAD检测逻辑简化 - 主要的静音监控现在由monitorVADSilence goroutine处理
						// 这里只做基本的语音检测来重置静音计时器
						if vadWakeWordDetected && c.GetState() == client.StateListening && enableVAD {
							if audioManager.VAD() != nil && audioManager.IsVADSpeech() {
								// 检测到语音，重置静音计时器
								if !vadSilenceStartTime.IsZero() {
									logrus.Debugf("🎯 VAD模式：检测到语音活动，重置静音计时器")
								}
								vadSilenceStartTime = time.Time{}
							}
						}
					})

					// 开始持续录音 - the audio manager runs continuously
					// Audio data will be sent to server only when in active recording state
					if err := audioManager.StartRecording(); err != nil {
						logrus.Errorf("启动持续录音失败: %v", err)
					} else {
						logrus.Info("持续录音已启动，用于唤醒词检测和语音输入")
					}
				}
			}
		}
	} else {
		// 如果没有启用唤醒词检测，设置默认的PCM数据回调 and audio data callback for normal operation
		if audioManager != nil {
			audioManager.AddPCMDataCallback("normal_mode_pcm", func(data []int16, size int) {
				// 复制数据以避免竞争条件
				dataCopy := make([]int16, size)
				copy(dataCopy, data[:size])
			})

			// In normal mode, add the audio data callback to always send to server when recording
			audioManager.AddAudioDataCallback("normal_mode_sender", func(data []byte) {
				// logrus.Debugf("音频数据回调被调用（正常模式），大小: %d 字节，sendToServer: %v, audioChan: %v", len(data), sendToServer, audioChan != nil)
				// For normal mode, send data to server if we're in recording state
				if audioChan != nil && sendToServer {
					logrus.Debugf("接收到音频数据，大小: %d 字节，发送到服务器", len(data))

					// 发送到通道，不阻塞
					select {
					case audioChan <- data:
						logrus.Debugf("音频数据已发送到通道，大小: %d 字节", len(data))
					default:
						// 通道已满，丢弃此数据包
						logrus.Warn("音频数据通道已满，丢弃数据包")
					}
				} else {
					// When not recording, just log for debugging
					logrus.Debugf("音频数据已接收但未发送到服务器（当前非录音状态），sendToServer: %v, audioChan: %v", sendToServer, audioChan != nil)
				}
			})

			// Add PCM callback for VAD in normal mode
			audioManager.AddPCMDataCallback("vad_normal_mode", func(data []int16, size int) {
				// 使用集成的VAD检测器处理音频数据
				if audioManager.VAD() != nil {
					if err := audioManager.ProcessVADAudio(data); err != nil && debugEnabled {
						logrus.Debugf("VAD处理音频数据失败: %v", err)
					}
				}

				// VAD检测逻辑简化 - 主要的静音监控现在由monitorVADSilence goroutine处理
				// 这里只做基本的语音检测来重置静音计时器
				if vadWakeWordDetected && c.GetState() == client.StateListening && enableVAD {
					if audioManager.VAD() != nil && audioManager.IsVADSpeech() {
						// 检测到语音，重置静音计时器
						if !vadSilenceStartTime.IsZero() {
							logrus.Debugf("🎯 VAD模式：检测到语音活动，重置静音计时器")
						}
						vadSilenceStartTime = time.Time{}
					}
				}
			})
		}
	}

	// 连接服务器
	err := proto.Connect(serverURL)
	if err != nil {
		logrus.Errorf("❌ 连接失败: %v", err)
		analyzeConnectionError(err)
		return
	}

	// 创建心跳定时器
	pingTicker := time.NewTicker(30 * time.Second)
	defer pingTicker.Stop()

	// 主循环
	for {
		select {
		case cmd := <-commandCh:
			logrus.Debugf("主循环收到命令: %s", cmd)
			// 直接处理简单的退出命令
			if cmd == "quit" || cmd == "q" {
				logrus.Info("收到退出命令，准备退出程序...")
				c.CloseAudioChannel()
				cleanupAndExit(c, 0)
			} else {
				logrus.Warnf("不支持的命令: %s", cmd)
			}

		case key := <-keyPressCh:
			// 处理按键事件
			logrus.Debugf("主循环收到按键事件: %s", key)
			handleKeyPress(c, key, &isRecording)

		case <-pingTicker.C:
			// 发送心跳包，保持连接
			if proto.IsConnected() {
				pingMsg := map[string]interface{}{
					"type": "ping",
					"id":   time.Now().Unix(),
				}

				if err := proto.SendJSON(pingMsg); err != nil {
					logrus.Warnf("发送心跳包失败: %v", err)
				}
			}
		}
	}
}

// safeExecute 安全执行函数，防止阻塞主循环
func safeExecute(fn func(), name string) {
	done := make(chan struct{})

	go func() {
		defer func() {
			if r := recover(); r != nil {
				logrus.Errorf("%s过程中发生异常: %v", name, r)
			}
			close(done)
		}()

		fn()
	}()

	// 不等待执行完成，继续主循环
	// 这只是为了捕获panic并记录日志
}

// handleKeyPress 处理按键事件，抽取为单独函数以便安全执行
func handleKeyPress(c *client.Client, key string, isRecording *bool) {
	// 在唤醒词模式下，仍然允许 'F' 和 'S' 按键功能
	// if enableWakeWord {
	// 	if key == "F2_PRESSED" {
	// 		logrus.Info("唤醒词检测已启用，手动录音按键被禁用")
	// 	}
	// 	return
	// }

	if key == "F2_PRESSED" && !*isRecording {
		// 先检查客户端是否已连接到服务器
		if !c.GetProtocol().IsConnected() {
			logrus.Error("客户端未连接到服务器，无法开始录音")
			fmt.Println("⚠️ 未连接到服务器，请先使用/connect命令连接")
			return
		}

		*isRecording = true
		logrus.Info("开始录音...")

		// 检查客户端当前状态
		currentState := c.GetState()
		logrus.Info("当前客户端状态:", currentState)
		if currentState == client.StateSpeaking {
			logrus.Info("正在中断AI回复以开始录音...")
			c.SendAbortSpeaking("start_recording")

			// 停止音频播放
			stopAudioPlayback(c)

		}

		if currentState != client.StateListening {
			// 如果客户端不在监听状态，先发送开始监听命令
			// 增加超时保护
			commandDone := make(chan error, 1)
			go func() {
				err := c.SendStartListening(client.ListenModeManual)
				commandDone <- err
			}()

			// 等待命令完成或超时
			var err error
			select {
			case err = <-commandDone:
				// 命令已完成
			case <-time.After(3 * time.Second):
				err = fmt.Errorf("发送开始监听命令超时")
				logrus.Error("发送开始监听命令超时")
			}

			if err != nil {
				logrus.Errorf("发送开始监听命令失败: %v", err)
				*isRecording = false
				fmt.Println("⚠️ 开始录音失败，请检查连接状态")
			} else {
				// 等待客户端状态变为监听状态
				waitStart := time.Now()
				stateChanged := false

				for time.Since(waitStart) < 2*time.Second {
					currentState = c.GetState()
					if currentState == client.StateListening {
						stateChanged = true
						break
					}
					time.Sleep(100 * time.Millisecond)
				}

				// 确认状态是否已变更
				if !stateChanged {
					logrus.Error("等待客户端进入监听状态超时")
					*isRecording = false
					fmt.Println("⚠️ 客户端进入监听状态超时")
				} else {
					// 现在开始录音
					startRecording(c)
				}
			}
		} else {
			// 客户端已经在监听状态，直接开始录音
			startRecording(c)
		}
	} else if key == "F2_RELEASED" {
		// 检查客户端当前状态，如果是Speaking状态，则停止播放
		currentState := c.GetState()
		if currentState == client.StateSpeaking {
			logrus.Info("正在中断AI回复...")
			if err := c.SendAbortSpeaking("stop_speaking"); err != nil {
				logrus.Errorf("发送停止讲话命令失败: %v", err)
			}

			// 停止音频播放
			stopAudioPlayback(c)
		}

		// 无论是否在录音状态，都处理F2_RELEASED事件
		if *isRecording {
			*isRecording = false
			logrus.Info("停止录音...")

			// 检查是否已连接到服务器
			if !c.GetProtocol().IsConnected() {
				logrus.Error("客户端未连接到服务器，但尝试停止录音")
				fmt.Println("⚠️ 连接已断开，无法正常停止录音")

				// 即使未连接，也要尝试停止本地录音设备
				if audioManager != nil {
					if err := audioManager.StopRecording(); err != nil {
						logrus.Errorf("停止录音失败: %v", err)
					}
				}

				// 清理音频通道
				if audioChan != nil {
					time.Sleep(50 * time.Millisecond)
					close(audioChan)
					audioChan = nil
				}
				return
			}

			// 停止录音 - use the stopRecording function that properly handles both normal and wake word modes
			stopRecording(c)
		}
	}
}

// initAudio 初始化音频系统
func initAudio() {
	var err error

	logrus.Debug("开始初始化音频系统...")

	// 创建音频管理器选项
	audioOptions := audio.AudioManagerOptions{
		SampleRate:        16000,
		ChannelCount:      1,
		FrameDuration:     60,
		UseDefaultDevices: true,
	}

	// 只有启用VAD时才创建VAD配置
	var vadConfig *audio.VADConfig
	if enableVAD {
		vadConfig = audio.NewVADConfig()
		vadConfig.Enabled = true
		vadConfig.Threshold = 0.5
		vadConfig.MinSilenceDuration = 0.5
		vadConfig.MinSpeechDuration = 0.25
		vadConfig.MaxSpeechDuration = 10.0
		vadConfig.WindowSize = 512
		vadConfig.SampleRate = 16000
		vadConfig.SilenceTimeout = time.Duration(silenceTimeoutMs) * time.Millisecond
		vadConfig.Debug = debugEnabled
		audioOptions.VADConfig = vadConfig
		logrus.Info("高级VAD语音检测已启用")
	} else {
		logrus.Info("使用简单能量阈值检测，高级VAD功能已禁用")
	}

	// 始终初始化音频管理器，包括播放功能
	// 即使启用唤醒词检测，我们仍需要音频管理器来 record user commands after wake word detection
	audioManager, err = audio.NewAudioManagerWithOptions(audioOptions)
	if err != nil {
		logrus.Warnf("初始化音频管理器失败: %v，将无法录音和播放", err)
	} else {
		logrus.Debug("音频管理器初始化成功")
		if enableVAD && audioManager.VAD() != nil {
			logrus.Info("高级VAD语音检测器已初始化并集成到音频管理器")
		} else if enableVAD {
			logrus.Warn("VAD功能已启用但检测器未初始化，将使用简单能量阈值检测")
		}
	}

	// audioPlayer 的初始化全部移除，防止oto.NewContext多次调用

	logrus.Info("音频系统初始化完成")
}

// cleanupAudio 清理音频系统资源
func cleanupAudio() {
	if audioManager != nil && audioManager.Player() != nil {
		if err := audioManager.Player().Close(); err != nil {
			logrus.Errorf("关闭音频播放器失败: %v", err)
		}
	}

	if audioManager != nil {
		if err := audioManager.Close(); err != nil {
			logrus.Errorf("关闭音频管理器失败: %v", err)
		}
	}

	// 关闭音频数据通道
	if audioChan != nil {
		logrus.Debug("关闭音频数据通道...")
		time.Sleep(50 * time.Millisecond)
		close(audioChan)
		audioChan = nil
	}
}

// stopAudioPlayback 停止音频播放
func stopAudioPlayback(c *client.Client) {
	// 先等待500毫秒，给音频播放器一些时间处理缓冲区中的数据
	logrus.Debug("等待500毫秒后停止音频播放...")
	time.Sleep(500 * time.Millisecond)

	// 停止音频播放
	if audioManager != nil && audioManager.Player() != nil && audioManager.Player().IsPlaying() {
		if err := audioManager.Player().Stop(); err != nil {
			logrus.Errorf("停止音频播放失败: %v", err)
		} else {
			logrus.Info("已停止音频播放")
		}
	}
}

// runActivation 运行激活流程
func runActivation() {
	logrus.Info("开始执行设备激活流程...")

	// 创建OTA客户端
	otaClient := ota.NewOTAClient(deviceID, appVersion, boardType)

	// 请求激活
	resp, err := otaClient.RequestActivation()
	if err != nil {
		logrus.Fatalf("设备激活失败: %v", err)
	}

	logrus.Info("设备激活成功!")
	logrus.Infof("激活码: %s", resp.Activation.Code)
	logrus.Infof("固件版本: %s", resp.Firmware.Version)
	logrus.Infof("MQTT配置: 端点=%s, 客户端ID=%s",
		resp.MQTT.Endpoint, resp.MQTT.ClientID)
}

// setupCallbacks 设置客户端回调
func setupCallbacks(c *client.Client) {
	// 状态变更回调
	c.SetOnStateChanged(func(oldState, newState string) {
		logrus.Infof("客户端状态变更: %s -> %s", oldState, newState)

		// 处理不同的状态变更
		if oldState != StateListening && newState == StateListening {
			// 进入监听状态，开始录音
			// Only start recording if not using wake word detection,
			// because for wake word detection, we start recording manually after detection
			if !enableWakeWord {
				startRecording(c)
			}
		} else if oldState == StateListening && newState != StateListening {
			// 退出监听状态，停止录音
			stopRecording(c)
		}
	})

	// 网络错误回调
	c.SetOnNetworkError(func(err error) {
		logrus.Errorf("网络错误: %v", err)
	})

	// 识别文本回调
	c.SetOnRecognizedText(func(text string) {
		logrus.Infof("识别到文本: %s", text)
	})

	// 朗读文本回调
	c.SetOnSpeakText(func(text string) {
		logrus.Infof("AI回复: %s", text)
	})

	// 音频数据回调
	c.SetOnAudioData(func(data []byte) {
		// logrus.Debugf("收到音频数据: %d字节", len(data))
		// 将音频数据添加到播放队列
		if audioManager != nil && audioManager.Player() != nil {
			audioManager.Player().QueueAudio(data)
			if audioManager.Player().IsDummyMode() {
				// 如果是哑模式，简单记录一下
				logrus.Debugf("音频在哑模式下处理")
			}
		}
	})

	// 情感变更回调
	c.SetOnEmotionChanged(func(emotion, text string) {
		logrus.Infof("情感变更: %s, 表情: %s", emotion, text)
	})

	// IoT命令回调
	c.SetOnIoTCommand(func(commands []interface{}) {
		logrus.Infof("收到IoT命令: %v", commands)
		// 这里可以实现IoT命令处理
	})

	// 音频通道打开回调
	c.SetOnAudioChannelOpen(func() {
		logrus.Info("音频通道已打开")
	})

	// 音频通道关闭回调
	c.SetOnAudioChannelClosed(func() {
		logrus.Info("音频通道已关闭")
		// 如果正在录音，停止录音
		stopRecording(c)
	})
}

// startRecording 开始录音 (for sending to server after wake word detection)
func startRecording(c *client.Client) {
	logrus.Debug("开始录音流程（发送到服务器）")

	// 添加调用来源信息，以便调试
	pc, file, line, ok := runtime.Caller(1)
	callerInfo := "unknown"
	if ok {
		funcName := runtime.FuncForPC(pc).Name()
		fileName := filepath.Base(file)
		callerInfo = fmt.Sprintf("%s:%d (%s)", fileName, line, funcName)
	}
	logrus.Infof("🎤 startRecording 调用来源: %s", callerInfo)

	if audioManager == nil {
		logrus.Error("音频管理器未初始化，无法录音")
		return
	}

	currentState := "unknown"
	if c != nil {
		currentState = c.GetState()
	}
	logrus.Infof("🎤 startRecording: 客户端当前状态: %s, enableWakeWord: %v", currentState, enableWakeWord)

	// 如果客户端不在监听状态，确保先发送开始监听命令
	if c != nil && c.GetState() != client.StateListening {
		logrus.Info("🎤 startRecording: 客户端不在监听状态，发送开始监听命令...")
		if err := c.SendStartListening(client.ListenModeManual); err != nil {
			logrus.Errorf("发送开始监听命令失败: %v", err)
			return
		}
		logrus.Info("已向服务器发送开始监听命令")
	}

	// 检查是否已经在录音状态，避免重复设置
	if audioChan != nil {
		logrus.Info("🎤 startRecording: 录音通道已存在，激活发送到服务器")
		// Just ensure we're sending to server
		oldSendToServer := sendToServer
		sendToServer = true
		if oldSendToServer != sendToServer {
			logrus.Info("🎤 startRecording: 已激活音频数据发送到服务器")
		}
		return
	}

	logrus.Info("🎤 startRecording: 创建新的录音数据通道")
	// 创建一个带缓冲的通道来接收音频数据
	audioChan = make(chan []byte, 100) // 足够大的缓冲区

	// 启动一个单独的goroutine处理音频数据发送
	go func() {
		logrus.Info("🎤 startRecording: 音频数据发送goroutine已启动")
		packetCount := 0
		for data := range audioChan {
			packetCount++
			logrus.Debugf("从通道接收到音频数据，准备发送到服务器，大小: %d 字节 (包 #%d)", len(data), packetCount)

			// 发送音频数据到服务器
			startTime := time.Now()
			err := c.SendAudioData(data)
			elapsed := time.Since(startTime)

			if err != nil {
				logrus.Errorf("发送音频数据失败: %v", err)
			} else {
				logrus.Debugf("音频数据已成功发送到服务器，大小: %d 字节，耗时: %v (包 #%d)", len(data), elapsed, packetCount)
				if elapsed > 100*time.Millisecond {
					logrus.Warnf("发送音频数据耗时较长: %v，数据大小: %d字节 (包 #%d)", elapsed, len(data), packetCount)
				}
			}
		}
		logrus.Infof("🎤 startRecording: 音频数据处理已停止，总共处理了 %d 个数据包", packetCount)
	}()

	// Enable sending audio data to server
	oldSendToServer := sendToServer
	sendToServer = true
	logrus.Infof("🎤 startRecording: 已设置 sendToServer = true (之前: %v)，当前状态: %s", oldSendToServer, c.GetState())

	// In wake word mode, audio manager is already running for wake word detection
	// We should not call StartRecording again as it's already running
	// Only start if not in wake word mode and not already recording
	if !enableWakeWord && !audioManager.IsRecording() {
		logrus.Info("🎤 startRecording: 启动音频管理器录音...")
		if err := audioManager.StartRecording(); err != nil {
			logrus.Errorf("开始录音失败: %v，将无法发送语音", err)
			// Clean up if we can't start recording
			if audioChan != nil {
				close(audioChan)
				audioChan = nil
			}
			sendToServer = false
			return
		} else {
			logrus.Info("音频管理器录音已启动")
		}
	} else {
		logrus.Debugf("🎤 startRecording: 音频管理器已在录音中 (%v) 或处于唤醒词检测模式 (%v)", audioManager.IsRecording(), enableWakeWord)
	}

	logrus.Info("🎤 startRecording: 录音数据发送已激活，等待音频输入...")
}

// stopRecording 停止录音并发送停止监听消息到服务器
func stopRecording(c *client.Client) {
	if audioManager == nil {
		return
	}

	// 停止向服务器发送音频数据
	// 关闭音频数据通道
	if audioChan != nil {
		logrus.Debug("关闭录音数据通道")
		time.Sleep(50 * time.Millisecond)
		close(audioChan)
		audioChan = nil
	} else {
		logrus.Debug("录音数据通道已为nil，无需关闭")
	}

	// Disable sending audio data to server
	sendToServer = false
	logrus.Debug("已设置 sendToServer = false")

	// Note: In wake word mode, we don't stop the audio manager recording
	// because it needs to continue for background wake word detection
	// Only stop when not using wake word detection
	if !enableWakeWord {
		if err := audioManager.StopRecording(); err != nil {
			logrus.Errorf("停止录音失败: %v", err)
		} else {
			logrus.Info("已停止录音")
		}
	} else {
		logrus.Info("唤醒词检测模式：保持音频输入运行以继续检测唤醒词")

		// 停止唤醒词录音定时器（如果存在）
		if wakeWordTimer != nil {
			wakeWordTimer.Stop()
			wakeWordTimer = nil
			logrus.Debug("已停止唤醒词录音定时器")
		}

		// 重置VAD标志，准备下一次录音
		vadWakeWordDetected = false
		vadSilenceStartTime = time.Time{}
		logrus.Debug("已重置VAD标志")
	}

	// 向服务器发送停止监听的消息
	if c != nil {
		currentState := c.GetState()
		if currentState == client.StateListening {
			if err := c.SendStopListening(); err != nil {
				logrus.Errorf("发送停止监听消息失败: %v", err)
			} else {
				logrus.Info("已向服务器发送停止监听消息")
			}
		}
	}
}

// triggerAutoInteraction 触发自动交互模式
func triggerAutoInteraction(c *client.Client) {
	logrus.Info("🚀 自动交互模式触发开始...")

	// 检查客户端是否已连接到服务器
	if !c.GetProtocol().IsConnected() {
		logrus.Warn("自动交互模式：客户端未连接到服务器，跳过自动录音")
		return
	}
	logrus.Info("✅ 客户端连接正常")

	// 检查当前状态
	currentState := c.GetState()
	logrus.Infof("🔍 自动交互模式：当前客户端状态: %s", currentState)

	// 如果正在播放AI回复，等待一小段时间看是否会更新状态
	if currentState == client.StateSpeaking {
		logrus.Info("⏸️ 自动交互模式：检测到speaking状态，等待状态更新...")
		// 等待最多1秒看状态是否会自动更新
		for i := 0; i < 10; i++ {
			time.Sleep(100 * time.Millisecond)
			currentState = c.GetState()
			if currentState != client.StateSpeaking {
				logrus.Infof("✅ 自动交互模式：状态已更新为: %s", currentState)
				break
			}
		}

		// 如果等待后仍然是speaking状态，手动设置为idle
		if currentState == client.StateSpeaking {
			logrus.Info("⚠️ 自动交互模式：状态未自动更新，手动设置为idle")
			c.SetState(client.StateIdle)
			currentState = client.StateIdle
		}
	}

	// 如果已经在监听状态，无需重复启动
	if currentState == client.StateListening {
		logrus.Info("🔄 自动交互模式：已在监听状态，无需重复启动")
		return
	}

	// 延迟一小段时间，确保TTS完全播放完毕
	logrus.Debug("⏳️ 自动交互模式：等待TTS完全播放完毕...")
	time.Sleep(500 * time.Millisecond)

	// 再次检查状态，确保在延迟期间没有变化
	currentState = c.GetState()
	logrus.Infof("🔍 自动交互模式：延迟后客户端状态: %s", currentState)
	if currentState == client.StateSpeaking {
		logrus.Info("⏸️ 自动交互模式：状态已变更，AI正在回复，跳过自动录音")
		return
	}
	if currentState == client.StateListening {
		logrus.Info("🔄 自动交互模式：状态已变更，已在监听状态，无需重复启动")
		return
	}

	// 发送开始监听命令到服务器
	logrus.Info("🎤 自动交互模式：发送开始监听命令到服务器...")
	if err := c.SendStartListening(client.ListenModeAuto); err != nil {
		logrus.Errorf("❌ 自动交互模式：发送开始监听命令失败: %v", err)
		return
	}
	logrus.Info("✅ 自动交互模式：开始监听命令发送成功")

	// 等待状态变更
	logrus.Info("⏳️ 自动交互模式：等待状态变更为监听状态...")
	stateChangeTimeout := time.After(3 * time.Second)
	for {
		currentState = c.GetState()
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

	// 开始录音
	logrus.Info("🎤 自动交互模式：开始录音...")
	startRecording(c)

	// 验证录音是否已启动
	if sendToServer && audioChan != nil {
		logrus.Info("✅ 自动交互模式：录音已启动，音频通道已创建")
	} else {
		logrus.Warn("⚠️ 自动交互模式：录音启动验证失败")
	}

	// 如果启用了唤醒词检测，启用VAD静音检测
	if enableWakeWord {
		logrus.Info("🎯 自动交互模式：唤醒词检测已启用，启用VAD静音检测")

		// 启用VAD功能，设置全局标志
		vadWakeWordDetected = true
		vadSilenceStartTime = time.Time{} // 重置静音开始时间

		logrus.Infof("✅ 自动交互模式：VAD静音检测已启用，%d秒静音后自动停止录音", autoInteractionSilenceThreshold)

		// 启动VAD监控goroutine
		go monitorVADSilence(c)

		// 设置10秒超时定时器作为备用机制
		const maxRecordingDuration = 10 * time.Second
		if wakeWordTimer != nil {
			wakeWordTimer.Stop()
		}
		wakeWordTimer = time.AfterFunc(maxRecordingDuration, func() {
			if vadWakeWordDetected && c.GetState() == client.StateListening {
				logrus.Info("🎯 自动交互模式：录音达到最大时长10秒，自动停止录音...")
				stopRecording(c)
				vadWakeWordDetected = false
			}
		})
		logrus.Infof("🎯 自动交互模式：已设置%d秒超时保护定时器", maxRecordingDuration/time.Second)
	} else {
		logrus.Info("🎤 自动交互模式：正常录音模式")
	}
}

// monitorVADSilence 监控VAD静音状态
func monitorVADSilence(c *client.Client) {
	logrus.Debug("VAD监控goroutine已启动")

	ticker := time.NewTicker(500 * time.Millisecond) // 每500毫秒检查一次
	defer ticker.Stop()

	lastSilenceSeconds := -1 // 记录上次输出的静音秒数，避免重复输出

	for {
		select {
		case <-ticker.C:
			// 检查VAD是否还在激活状态
			if !vadWakeWordDetected {
				logrus.Debug("VAD监控: vadWakeWordDetected为false，退出监控")
				return
			}

			// 检查客户端状态
			if c.GetState() != client.StateListening {
				continue
			}

			// 获取VAD静音持续时间
			var silenceDuration time.Duration
			var hasSpeech, isSilence bool

			if audioManager != nil && audioManager.VAD() != nil && enableVAD {
				// 使用集成的VAD检测结果
				silenceDuration = audioManager.GetVADSilenceDuration()
				hasSpeech = audioManager.IsVADSpeech()
				isSilence = audioManager.IsVADSilence()
			} else {
				// 使用本地计时
				if !vadSilenceStartTime.IsZero() {
					silenceDuration = time.Since(vadSilenceStartTime)
				}
				isSilence = true
			}

			// 如果检测到语音，重置静音计时器
			if hasSpeech {
				if !vadSilenceStartTime.IsZero() {
					logrus.Debugf("🎯 VAD监控：检测到语音活动，重置静音计时器（之前静音了%.1f秒）", silenceDuration.Seconds())
				}
				vadSilenceStartTime = time.Time{}
				lastSilenceSeconds = -1
				continue
			}

			// 如果是静音状态，记录静音开始时间
			if isSilence && vadSilenceStartTime.IsZero() {
				vadSilenceStartTime = time.Now()
				logrus.Debugf("🎯 VAD监控：检测到静音，开始记录静音时间，静音开始于: %v", vadSilenceStartTime.Format("15:04:05.000"))
			}

			// 计算静音持续时间
			if silenceDuration > 0 {
				silenceSeconds := int(silenceDuration.Seconds())

				// 根据录音模式设置不同的静音超时时间
				var silenceThreshold time.Duration
				var modeDesc string

				// 检查是否是自动交互模式
				if autoInteraction && c.GetState() == client.StateListening && sendToServer {
					silenceThreshold = time.Duration(autoInteractionSilenceThreshold) * time.Second
					modeDesc = "自动交互模式"
				} else {
					silenceThreshold = 3 * time.Second
					modeDesc = "唤醒词模式"
				}

				// 每秒输出一次静音时长日志，避免刷屏
				if silenceSeconds != lastSilenceSeconds && silenceSeconds > 0 {
					logrus.Infof("🎯 %s：持续静音时间: %.1f秒，阈值: %.1f秒", modeDesc, silenceDuration.Seconds(), float64(silenceThreshold)/float64(time.Second))
					lastSilenceSeconds = silenceSeconds
				}

				// 检查是否达到静音阈值
				if silenceDuration >= silenceThreshold {
					logrus.Infof("✅ 连续%.1f秒检测不到声音，自动停止录音（%s，等同于按S键）...",
						float64(silenceThreshold)/float64(time.Second), modeDesc)

					// 停止录音（等同于按S键）
					stopRecording(c)

					// 重置标志
					vadWakeWordDetected = false
					vadSilenceStartTime = time.Time{}
					lastSilenceSeconds = -1

					logrus.Debug("VAD监控: 静音超时，停止录音，退出监控")
					return
				}
			}
		}
	}
}

// generateUUID 基于MAC地址生成UUID
func generateUUID(macAddr string) string {
	// 如果MAC地址为空，使用随机数据
	var data []byte
	if macAddr == "" {
		data = make([]byte, 16)
		rand.Read(data)
	} else {
		// 使用MAC地址作为种子计算MD5
		h := md5.New()
		h.Write([]byte(macAddr))
		data = h.Sum(nil)
	}

	// 设置UUID版本 (版本4)
	data[6] = (data[6] & 0x0F) | 0x40
	// 设置变体
	data[8] = (data[8] & 0x3F) | 0x80

	// 按UUID格式转换为字符串
	return fmt.Sprintf("%x-%x-%x-%x-%x",
		data[0:4], data[4:6], data[6:8], data[8:10], data[10:16])
}

// getMACAddress 获取本机MAC地址
func getMACAddress() (string, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return "", err
	}

	for _, i := range interfaces {
		if i.Flags&net.FlagUp != 0 && i.Flags&net.FlagLoopback == 0 {
			if len(i.HardwareAddr) > 0 {
				return strings.ToLower(i.HardwareAddr.String()), nil
			}
		}
	}

	return "", fmt.Errorf("未找到有效的网络接口")
}

// isDeviceActivated 检查设备是否已激活
func isDeviceActivated() bool {
	// 创建OTA客户端
	otaClient := ota.NewOTAClient(deviceID, appVersion, boardType)

	// 检查激活状态
	activated, err := otaClient.CheckActivationStatus()
	if err != nil {
		logrus.Errorf("检查设备激活状态失败: %v", err)
		return false
	}

	return activated
}

// readInput 处理按键输入
func readInput(keyPressCh chan<- string, commandCh chan<- string) {
	// 设置终端为原始模式
	if err := exec.Command("stty", "-F", "/dev/tty", "cbreak", "min", "1").Run(); err != nil {
		logrus.Errorf("设置终端cbreak模式失败: %v", err)
	}
	// 关闭终端回显
	if err := exec.Command("stty", "-F", "/dev/tty", "-echo").Run(); err != nil {
		logrus.Errorf("关闭终端回显失败: %v", err)
	}

	// 即使在goroutine中发生panic，也要尝试恢复终端设置
	defer func() {
		if err := exec.Command("stty", "-F", "/dev/tty", "echo").Run(); err != nil {
			logrus.Errorf("恢复终端回显失败: %v", err)
		}
		if err := exec.Command("stty", "-F", "/dev/tty", "-cbreak").Run(); err != nil {
			logrus.Errorf("恢复终端规范模式失败: %v", err)
		}
	}()

	// 记录录音按键状态，防止重复触发
	recordKeyPressed := false

	for {
		var b [1]byte
		_, err := os.Stdin.Read(b[:])
		if err != nil {
			logrus.Errorf("读取输入失败: %v", err)
			continue
		}

		// 处理特殊命令，仅保留退出功能
		if b[0] == 'q' || b[0] == 'Q' {
			// 退出命令
			logrus.Info("准备退出程序")
			commandCh <- "quit"
			continue
		}

		// 处理录音相关按键
		switch b[0] {
		case 'f', 'F': // 按f开始录音
			if !recordKeyPressed {
				recordKeyPressed = true
				keyPressCh <- "F2_PRESSED"
			}
		case 's', 'S': // 按s停止录音
			if recordKeyPressed {
				recordKeyPressed = false
				keyPressCh <- "F2_RELEASED"
			}
		}
	}
}

// reinitializeOpusDecoder 重新初始化Opus解码器
func reinitializeOpusDecoder(sampleRate, channels, frameDuration int) {
	if sampleRate <= 0 || channels <= 0 || frameDuration <= 0 {
		logrus.Error("无效的音频参数，无法初始化Opus解码器")
		return
	}

	logrus.Infof("开始重新初始化Opus解码器: sample_rate=%d, channels=%d, frame_duration=%d",
		sampleRate, channels, frameDuration)

	if audioManager == nil {
		logrus.Error("audioManager未初始化，无法重新初始化解码器")
		return
	}

	if audioInited {
		logrus.Warn("检测到服务器音频参数变化，Oto 不支持热切换采样率，请重启程序以应用新参数！")
		return
	}

	err := audioManager.RecreatePlayer(sampleRate, channels, frameDuration)
	if err != nil {
		logrus.Errorf("重建播放器失败: %v", err)
	} else {
		audioManager.Player().Start()
		logrus.Info("已根据服务器参数重建播放器")
		audioInited = true
	}
}
