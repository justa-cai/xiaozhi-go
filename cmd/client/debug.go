package main

import (
	"os"
	"runtime"
	"runtime/pprof"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

var debugEnabled = false

// EnableDebug 启用高级调试功能
func EnableDebug() {
	debugEnabled = true
	logrus.Info("高级调试功能已启用")
}

// DumpGoroutines 输出所有goroutine的堆栈信息到日志
func DumpGoroutines() {
	if !debugEnabled {
		return
	}

	logrus.Info("=== 开始转储goroutine堆栈 ===")
	buf := make([]byte, 1<<20)
	stackLen := runtime.Stack(buf, true)

	// 将堆栈信息分段记录，防止日志过长
	stackStr := string(buf[:stackLen])
	chunks := strings.Split(stackStr, "\n")

	for _, line := range chunks {
		if line != "" {
			logrus.Info(line)
		}
	}
	logrus.Info("=== goroutine堆栈转储结束 ===")
}

// CPUProfile 创建一个CPU分析文件
func CPUProfile(duration time.Duration) {
	if !debugEnabled {
		return
	}

	logrus.Infof("开始收集CPU分析数据，持续%v...", duration)

	f, err := os.Create("cpu_profile.prof")
	if err != nil {
		logrus.Errorf("创建CPU分析文件失败: %v", err)
		return
	}

	if err := pprof.StartCPUProfile(f); err != nil {
		logrus.Errorf("启动CPU分析失败: %v", err)
		f.Close()
		return
	}

	// 在指定时间后停止分析
	go func() {
		time.Sleep(duration)
		pprof.StopCPUProfile()
		f.Close()
		logrus.Info("CPU分析数据收集完成，已保存到cpu_profile.prof")
	}()
}

// HeapProfile 创建一个堆内存分析文件
func HeapProfile() {
	if !debugEnabled {
		return
	}

	logrus.Info("收集堆内存分析数据...")

	f, err := os.Create("heap_profile.prof")
	if err != nil {
		logrus.Errorf("创建堆内存分析文件失败: %v", err)
		return
	}
	defer f.Close()

	if err := pprof.WriteHeapProfile(f); err != nil {
		logrus.Errorf("写入堆内存分析数据失败: %v", err)
		return
	}

	logrus.Info("堆内存分析数据已保存到heap_profile.prof")
}

// StartAudioMonitor 启动音频系统监控
func StartAudioMonitor() chan struct{} {
	stopCh := make(chan struct{})

	if !debugEnabled {
		return stopCh
	}

	logrus.Info("启动音频系统监控...")

	// 每秒检查一次音频系统状态
	ticker := time.NewTicker(1 * time.Second)

	go func() {
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				logrus.Debug("音频系统监控运行中...")
			case <-stopCh:
				logrus.Info("音频系统监控已停止")
				return
			}
		}
	}()

	return stopCh
}
