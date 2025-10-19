package config

import (
	"flag"
	"fmt"
)

// Flags 配置结构体
type Flags struct {
	ServerURL                   string
	DeviceID                    string
	Token                       string
	BoardType                   string
	AppVersion                  string
	ActivateOnly                bool
	LogLevel                    string
	SkipTLSVerify               bool
	HTTPProxy                   string
	DebugEnabled                bool
	VerboseLogging              bool
	EnableWakeWord              bool
	AutoInteraction             bool
	EnableVAD                   bool
	SilenceTimeoutMs            int
	AutoInteractionSilenceThreshold int
}

// ParseFlags 解析命令行参数
func ParseFlags() *Flags {
	flags := &Flags{}

	// 解析命令行参数
	flag.StringVar(&flags.ServerURL, "server", DefaultServerURL, "WebSocket服务器地址")
	flag.StringVar(&flags.DeviceID, "device-id", DefaultDeviceID, "设备ID (MAC地址)")
	flag.StringVar(&flags.Token, "token", DefaultToken, "API访问令牌")
	flag.StringVar(&flags.BoardType, "board", DefaultBoardType, "设备板型号")
	flag.StringVar(&flags.AppVersion, "version", DefaultAppVersion, "应用版本号")
	flag.BoolVar(&flags.ActivateOnly, "activate-only", false, "只执行激活流程")
	flag.StringVar(&flags.LogLevel, "log-level", DefaultLogLevel, "日志级别 (debug, info, warn, error, fatal, panic)")
	flag.BoolVar(&flags.SkipTLSVerify, "skip-tls-verify", true, "跳过TLS证书验证")
	flag.StringVar(&flags.HTTPProxy, "http-proxy", "", "HTTP代理地址，例如: http://127.0.0.1:8080")

	// 调试和日志相关
	flag.BoolVar(&flags.DebugEnabled, "debug", false, "启用高级调试功能")
	flag.BoolVar(&flags.VerboseLogging, "verbose", false, "启用详细日志")

	// 唤醒词和交互相关
	flag.BoolVar(&flags.EnableWakeWord, "wakeword", false, "启用唤醒词检测功能")
	flag.BoolVar(&flags.AutoInteraction, "auto-interaction", true, "启用自动交互模式（TTS播放结束后自动开始录音）")

	// VAD相关
	flag.BoolVar(&flags.EnableVAD, "vad", true, "启用高级语音活动检测(VAD)功能，提供更准确的人声检测")
	flag.IntVar(&flags.SilenceTimeoutMs, "silence-timeout", DefaultSilenceTimeout, "静音超时时间（毫秒），超过此时间无语音则自动停止录音")
	flag.IntVar(&flags.AutoInteractionSilenceThreshold, "auto-silence-threshold", DefaultAutoSilenceThreshold, "自动交互模式静音时间阈值（秒），超过此时间无语音则自动停止录音")

	flag.Parse()

	return flags
}

// PrintHelp 打印帮助信息
func PrintHelp() {
	fmt.Println("小智客户端 - 语音助手客户端")
	fmt.Println()
	fmt.Println("用法:")
	fmt.Println("  client [选项]")
	fmt.Println()
	fmt.Println("选项:")
	flag.PrintDefaults()
	fmt.Println()
	fmt.Println("示例:")
	fmt.Println("  client -server ws://192.168.1.100:8080/ws -token my-token")
	fmt.Println("  client -wakeword -auto-interaction -vad")
	fmt.Println("  client -activate-only -device-id aa:bb:cc:dd:ee:ff")
}

// Validate 验证配置参数
func (f *Flags) Validate() error {
	// 这里可以添加配置验证逻辑
	// 例如检查端口范围、文件路径有效性等
	return nil
}

// GetAudioConfig 获取音频配置
func (f *Flags) GetAudioConfig() AudioConfig {
	return AudioConfig{
		SampleRate:        16000,
		ChannelCount:      1,
		FrameDuration:     60,
		UseDefaultDevices: true,
		EnableVAD:         f.EnableVAD,
		SilenceTimeoutMs:  f.SilenceTimeoutMs,
	}
}

// 实现 LogConfigGetter 接口
func (f *Flags) GetLogLevel() string {
	return f.LogLevel
}

func (f *Flags) IsVerboseLogging() bool {
	return f.VerboseLogging
}

func (f *Flags) IsDebugEnabled() bool {
	return f.DebugEnabled
}

// AudioConfig 音频配置
type AudioConfig struct {
	SampleRate        int
	ChannelCount      int
	FrameDuration     int
	UseDefaultDevices bool
	EnableVAD         bool
	SilenceTimeoutMs  int
}