package interaction

import (
	"fmt"
	"os"

	"github.com/justa-cai/xiaozhi-go/cmd/client/utils"
	"github.com/sirupsen/logrus"
)

// InputManager 用户输入管理器
type InputManager struct {
	keyPressCh   chan string
	commandCh    chan string
	terminalMgr  *utils.TerminalManager
	exitManager  *utils.ExitManager
	isRunning    bool
}

// NewInputManager 创建新的输入管理器
func NewInputManager() *InputManager {
	return &InputManager{
		keyPressCh:  make(chan string, 10),
		commandCh:   make(chan string, 10),
		terminalMgr: utils.GetGlobalTerminalManager(),
		exitManager: utils.GetGlobalExitManager(),
		isRunning:   false,
	}
}

// Start 启动输入监听
func (im *InputManager) Start() error {
	if im.isRunning {
		return nil
	}

	im.isRunning = true

	// 设置终端为原始模式
	if err := im.terminalMgr.SetRawMode(); err != nil {
		logrus.Errorf("设置终端原始模式失败: %v", err)
		return err
	}

	// 启动输入处理goroutine
	go im.processInput()

	logrus.Debug("输入管理器已启动")
	return nil
}

// Stop 停止输入监听
func (im *InputManager) Stop() {
	if !im.isRunning {
		return
	}

	im.isRunning = false

	// 恢复终端模式
	if err := im.terminalMgr.RestoreNormalMode(); err != nil {
		logrus.Errorf("恢复终端模式失败: %v", err)
	}

	// 关闭通道
	close(im.keyPressCh)
	close(im.commandCh)

	logrus.Debug("输入管理器已停止")
}

// GetKeyPressChannel 获取按键通道
func (im *InputManager) GetKeyPressChannel() <-chan string {
	return im.keyPressCh
}

// GetCommandChannel 获取命令通道
func (im *InputManager) GetCommandChannel() <-chan string {
	return im.commandCh
}

// processInput 处理输入
func (im *InputManager) processInput() {
	// 设置延迟恢复终端设置
	defer func() {
		if r := recover(); r != nil {
			logrus.Errorf("输入处理中发生异常: %v", r)
		}
		im.terminalMgr.RestoreNormalMode()
	}()

	// 记录录音按键状态，防止重复触发
	recordKeyPressed := false

	for im.isRunning {
		var b [1]byte
		_, err := os.Stdin.Read(b[:])
		if err != nil {
			if im.isRunning { // 只有在仍在运行时才记录错误
				logrus.Errorf("读取输入失败: %v", err)
			}
			continue
		}

		// 处理特殊命令
		if b[0] == 'q' || b[0] == 'Q' {
			// 退出命令
			logrus.Info("准备退出程序")
			select {
			case im.commandCh <- "quit":
			default:
				logrus.Warn("命令通道已满，丢弃退出命令")
			}
			continue
		}

		// 处理录音相关按键
		switch b[0] {
		case 'f', 'F': // 按f开始录音
			if !recordKeyPressed {
				recordKeyPressed = true
				select {
				case im.keyPressCh <- "F2_PRESSED":
				default:
					logrus.Warn("按键通道已满，丢弃按键事件")
				}
			}
		case 's', 'S': // 按s停止录音
			if recordKeyPressed {
				recordKeyPressed = false
				select {
				case im.keyPressCh <- "F2_RELEASED":
				default:
					logrus.Warn("按键通道已满，丢弃按键事件")
				}
			}
		}
	}
}

// IsRunning 检查是否正在运行
func (im *InputManager) IsRunning() bool {
	return im.isRunning
}

// SendCommand 发送命令（用于程序内部调用）
func (im *InputManager) SendCommand(cmd string) {
	if im.isRunning {
		select {
		case im.commandCh <- cmd:
		default:
			logrus.Warn("命令通道已满，丢弃命令: %s", cmd)
		}
	}
}

// SendKeyPress 发送按键事件（用于程序内部调用）
func (im *InputManager) SendKeyPress(key string) {
	if im.isRunning {
		select {
		case im.keyPressCh <- key:
		default:
			logrus.Warn("按键通道已满，丢弃按键事件: %s", key)
		}
	}
}

// SetupHelpDisplay 设置帮助信息显示
func (im *InputManager) SetupHelpDisplay(enableWakeWord, autoInteraction bool, autoSilenceThreshold int) {
	// 显示操作说明
	if enableWakeWord {
		fmt.Println("唤醒词检测模式已启用:")
		if autoInteraction {
			fmt.Printf("  🔄 自动交互模式已启用（TTS播放结束后自动开始录音，静音阈值：%d秒）\n", autoSilenceThreshold)
		}
		fmt.Println("  说出 '你好小智' 或 '小智同学' 来激活助手")
		fmt.Println("  f - 开始录音")
		fmt.Println("  s - 停止录音")
		fmt.Println("  q - 退出程序")
	} else {
		if autoInteraction {
			fmt.Printf("🔄 自动交互模式已启用（TTS播放结束后自动开始录音，静音阈值：%d秒）\n", autoSilenceThreshold)
		}
		fmt.Println("按键操作:")
		fmt.Println("  f - 开始录音")
		fmt.Println("  s - 停止录音")
		fmt.Println("  q - 退出程序")
	}
}