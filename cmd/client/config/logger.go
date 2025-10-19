package config

import (
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"
)

// LoggerConfig 日志配置
type LoggerConfig struct {
	Level           logrus.Level
	VerboseLogging  bool
	DebugEnabled    bool
	WebSocketHook   bool
}

// NewLoggerConfig 创建新的日志配置
func NewLoggerConfig(logLevel string, verboseLogging, debugEnabled bool) *LoggerConfig {
	level := parseLogLevel(logLevel)

	return &LoggerConfig{
		Level:           level,
		VerboseLogging:  verboseLogging,
		DebugEnabled:    debugEnabled,
		WebSocketHook:   true,
	}
}

// SetupLogger 设置日志系统
func (lc *LoggerConfig) SetupLogger() {
	// 设置日志级别
	logrus.SetLevel(lc.Level)

	// 设置日志格式
	logrus.SetFormatter(&logrus.TextFormatter{
		FullTimestamp:   true,
		TimestampFormat: "2006-01-02 15:04:05",
	})

	// 添加WebSocket连接日志钩子
	if lc.WebSocketHook {
		logrus.AddHook(&WebSocketLogHook{})
	}
}

// parseLogLevel 解析日志级别
func parseLogLevel(level string) logrus.Level {
	switch strings.ToLower(level) {
	case "debug":
		return logrus.DebugLevel
	case "info":
		return logrus.InfoLevel
	case "warn":
		return logrus.WarnLevel
	case "error":
		return logrus.ErrorLevel
	case "fatal":
		return logrus.FatalLevel
	case "panic":
		return logrus.PanicLevel
	default:
		logrus.Warnf("未知的日志级别: %s，使用默认级别 debug", level)
		return logrus.DebugLevel
	}
}

// WebSocketLogHook WebSocket连接日志钩子
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

// LogConfigGetter 日志配置获取接口
type LogConfigGetter interface {
	GetLogLevel() string
	IsVerboseLogging() bool
	IsDebugEnabled() bool
}

// SetupLoggerFromConfig 从配置设置日志
func SetupLoggerFromConfig(config LogConfigGetter) {
	loggerConfig := NewLoggerConfig(
		config.GetLogLevel(),
		config.IsVerboseLogging(),
		config.IsDebugEnabled(),
	)
	loggerConfig.SetupLogger()
}