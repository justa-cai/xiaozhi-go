package main

import (
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/justa-cai/xiaozhi-go/cmd/client/core"
	"github.com/justa-cai/xiaozhi-go/cmd/client/utils"
	"github.com/sirupsen/logrus"
)

func main() {
	// 创建应用程序实例
	app := core.NewApplication()

	// 设置信号处理
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		logrus.Infof("接收到信号: %v, 正在优雅退出...", sig)

		// 停止应用程序（带超时）
		if app.IsRunning() {
			done := make(chan struct{})
			go func() {
				app.Stop()
				close(done)
			}()

			// 等待最多2秒，如果超时则强制退出
			select {
			case <-done:
				logrus.Debug("应用程序已优雅停止")
			case <-time.After(2 * time.Second):
				logrus.Warn("应用程序停止超时，强制退出")
			}
		}

		// 安全退出
		utils.SafeExit(0)
	}()

	// 初始化应用程序
	if err := app.Initialize(); err != nil {
		logrus.Fatalf("应用程序初始化失败: %v", err)
	}

	// 运行应用程序
	if err := app.Run(); err != nil {
		logrus.Errorf("应用程序运行出错: %v", err)
		utils.SafeExit(1)
	}

	// 正常退出时确保清理
	if app.IsRunning() {
		app.Stop()
	}
	utils.SafeExit(0)
}