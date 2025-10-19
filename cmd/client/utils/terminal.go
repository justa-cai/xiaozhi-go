package utils

import (
	"os"
	"os/exec"
	"sync"

	"github.com/sirupsen/logrus"
)

// TerminalManager 终端状态管理器
type TerminalManager struct {
	originalState  *TerminalState
	mutex          sync.Mutex
	isRawMode      bool
	isInteractive  bool
}

// TerminalState 终端状态
type TerminalState struct {
	echo     bool
	cbreak   bool
	restored bool
}

// NewTerminalManager 创建新的终端管理器
func NewTerminalManager() *TerminalManager {
	// 检查是否为交互式环境
	isInteractive := isInteractiveTerminal()

	return &TerminalManager{
		isRawMode:     false,
		isInteractive: isInteractive,
	}
}

// isInteractiveTerminal 检查是否为交互式终端
func isInteractiveTerminal() bool {
	// 检查stdin是否为终端
	if fileInfo, _ := os.Stdin.Stat(); (fileInfo.Mode() & os.ModeCharDevice) == 0 {
		return false
	}

	// 检查/dev/tty是否存在
	if _, err := os.Stat("/dev/tty"); os.IsNotExist(err) {
		return false
	}

	return true
}

// SetRawMode 设置终端为原始模式（用于按键监听）
func (tm *TerminalManager) SetRawMode() error {
	tm.mutex.Lock()
	defer tm.mutex.Unlock()

	if !tm.isInteractive {
		logrus.Debug("非交互式环境，跳过终端原始模式设置")
		return nil
	}

	if tm.isRawMode {
		return nil // 已经是原始模式
	}

	// 保存当前状态（简单实现，实际应用中可能需要更复杂的状态保存）
	if tm.originalState == nil {
		tm.originalState = &TerminalState{
			echo:   true,  // 假设默认开启回显
			cbreak: false, // 假设默认不是cbreak模式
		}
	}

	// 设置终端为cbreak模式
	if err := exec.Command("stty", "-F", "/dev/tty", "cbreak", "min", "1").Run(); err != nil {
		logrus.Errorf("设置终端cbreak模式失败: %v", err)
		return err
	}

	// 关闭终端回显
	if err := exec.Command("stty", "-F", "/dev/tty", "-echo").Run(); err != nil {
		logrus.Errorf("关闭终端回显失败: %v", err)
		// 尝试恢复cbreak模式
		exec.Command("stty", "-F", "/dev/tty", "-cbreak").Run()
		return err
	}

	tm.isRawMode = true
	logrus.Debug("终端已设置为原始模式")
	return nil
}

// RestoreNormalMode 恢复终端为正常模式
func (tm *TerminalManager) RestoreNormalMode() error {
	tm.mutex.Lock()
	defer tm.mutex.Unlock()

	if !tm.isInteractive {
		logrus.Debug("非交互式环境，跳过终端模式恢复")
		return nil
	}

	if !tm.isRawMode {
		return nil // 已经是正常模式
	}

	// 恢复终端回显
	if err := exec.Command("stty", "-F", "/dev/tty", "echo").Run(); err != nil {
		logrus.Errorf("恢复终端回显失败: %v", err)
	}

	// 恢复终端规范模式
	if err := exec.Command("stty", "-F", "/dev/tty", "-cbreak").Run(); err != nil {
		logrus.Errorf("恢复终端规范模式失败: %v", err)
	}

	if tm.originalState != nil {
		tm.originalState.restored = true
	}

	tm.isRawMode = false
	logrus.Debug("终端已恢复为正常模式")
	return nil
}

// IsRawMode 检查是否为原始模式
func (tm *TerminalManager) IsRawMode() bool {
	tm.mutex.Lock()
	defer tm.mutex.Unlock()
	return tm.isRawMode
}

// SetupDeferredRestore 设置延迟恢复，通常在defer中使用
func (tm *TerminalManager) SetupDeferredRestore() {
	// 即使在goroutine中发生panic，也要尝试恢复终端设置
	defer func() {
		if r := recover(); r != nil {
			logrus.Errorf("终端操作中发生异常: %v", r)
		}
		tm.RestoreNormalMode()
	}()
}

// 全局终端管理器实例
var globalTerminalManager = NewTerminalManager()

// GetGlobalTerminalManager 获取全局终端管理器
func GetGlobalTerminalManager() *TerminalManager {
	return globalTerminalManager
}

// SetTerminalRawMode 设置终端为原始模式（全局函数）
func SetTerminalRawMode() error {
	return globalTerminalManager.SetRawMode()
}

// RestoreTerminalNormalMode 恢复终端为正常模式（全局函数）
func RestoreTerminalNormalMode() error {
	return globalTerminalManager.RestoreNormalMode()
}

// IsTerminalRawMode 检查终端是否为原始模式（全局函数）
func IsTerminalRawMode() bool {
	return globalTerminalManager.IsRawMode()
}