package utils

import (
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// ExitManager 安全退出管理器
type ExitManager struct {
	terminalRestored bool
	terminalMutex    sync.Mutex
	cleanupHandlers  []func()
}

// NewExitManager 创建新的退出管理器
func NewExitManager() *ExitManager {
	return &ExitManager{
		terminalRestored: false,
		cleanupHandlers:  make([]func(), 0),
	}
}

// AddCleanupHandler 添加清理处理器
func (em *ExitManager) AddCleanupHandler(handler func()) {
	em.cleanupHandlers = append(em.cleanupHandlers, handler)
}

// SafeExit 安全退出程序，确保恢复终端设置
func (em *ExitManager) SafeExit(code int) {
	// 快速路径：如果没有清理处理器且终端已恢复，直接退出
	if len(em.cleanupHandlers) == 0 && em.terminalRestored {
		os.Exit(code)
	}

	em.terminalMutex.Lock()
	defer em.terminalMutex.Unlock()

	if !em.terminalRestored {
		// 使用超时机制恢复终端设置，避免阻塞
		em.restoreTerminalWithTimeout()
		em.terminalRestored = true
		logrus.Debug("退出前已恢复终端设置")
	}

	// 执行清理处理器（带超时保护）
	em.executeCleanupHandlers()

	os.Exit(code)
}

// restoreTerminalWithTimeout 带超时的终端恢复
func (em *ExitManager) restoreTerminalWithTimeout() {
	// 检查是否为交互式环境
	if _, err := os.Stat("/dev/tty"); os.IsNotExist(err) {
		logrus.Debug("非交互式环境，跳过终端恢复")
		return
	}

	done := make(chan struct{})

	go func() {
		defer close(done)

		// 恢复终端设置
		if err := exec.Command("stty", "-F", "/dev/tty", "echo").Run(); err != nil {
			logrus.Errorf("退出时恢复终端回显失败: %v", err)
		}
		if err := exec.Command("stty", "-F", "/dev/tty", "-cbreak").Run(); err != nil {
			logrus.Errorf("退出时恢复终端规范模式失败: %v", err)
		}
	}()

	// 等待最多500ms
	select {
	case <-done:
		// 完成
	case <-time.After(500 * time.Millisecond):
		logrus.Warn("终端恢复超时，强制退出")
	}
}

// executeCleanupHandlers 执行清理处理器
func (em *ExitManager) executeCleanupHandlers() {
	// 如果没有注册任何处理器，直接返回
	if len(em.cleanupHandlers) == 0 {
		return
	}

	for _, handler := range em.cleanupHandlers {
		func() {
			defer func() {
				if r := recover(); r != nil {
					logrus.Errorf("清理处理器执行异常: %v", r)
				}
			}()

			// 为每个处理器设置超时
			done := make(chan struct{})
			go func() {
				defer close(done)
				handler()
			}()

			select {
			case <-done:
				// 处理器完成
			case <-time.After(1 * time.Second):
				logrus.Warn("清理处理器执行超时")
			}
		}()
	}
}

// RestoreTerminal 恢复终端设置
func (em *ExitManager) RestoreTerminal() {
	em.terminalMutex.Lock()
	defer em.terminalMutex.Unlock()

	if !em.terminalRestored {
		// 检查是否为交互式环境
		if _, err := os.Stat("/dev/tty"); os.IsNotExist(err) {
			logrus.Debug("非交互式环境，跳过终端恢复")
			em.terminalRestored = true
			return
		}

		if err := exec.Command("stty", "-F", "/dev/tty", "echo").Run(); err != nil {
			logrus.Errorf("恢复终端回显失败: %v", err)
		}
		if err := exec.Command("stty", "-F", "/dev/tty", "-cbreak").Run(); err != nil {
			logrus.Errorf("恢复终端规范模式失败: %v", err)
		}
		em.terminalRestored = true
		logrus.Debug("已恢复终端设置")
	}
}

// IsTerminalRestored 检查终端是否已恢复
func (em *ExitManager) IsTerminalRestored() bool {
	em.terminalMutex.Lock()
	defer em.terminalMutex.Unlock()
	return em.terminalRestored
}

// 全局退出管理器实例
var globalExitManager = NewExitManager()

// GetGlobalExitManager 获取全局退出管理器
func GetGlobalExitManager() *ExitManager {
	return globalExitManager
}

// SafeExit 全局安全退出函数
func SafeExit(code int) {
	globalExitManager.SafeExit(code)
}

// AddGlobalCleanupHandler 添加全局清理处理器
func AddGlobalCleanupHandler(handler func()) {
	globalExitManager.AddCleanupHandler(handler)
}

// RestoreTerminal 全局终端恢复函数
func RestoreTerminal() {
	globalExitManager.RestoreTerminal()
}