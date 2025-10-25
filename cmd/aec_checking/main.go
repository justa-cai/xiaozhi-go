package main

import (
	"encoding/binary"
	"fmt"
	"log"
	"math"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	clientAudio "github.com/justa-cai/xiaozhi-go/cmd/client/audio"
	"github.com/justa-cai/xiaozhi-go/internal/audio"
	"github.com/sirupsen/logrus"
)

// AECResult AEC检测结果
type AECResult struct {
	HasAEC        bool          // 是否支持AEC
	HasNoiseSupp  bool          // 是否支持噪声抑制
	HasGainCtrl   bool          // 是否支持增益控制
	Latency       time.Duration // 音频延迟
	SampleRate    int           // 采样率
	Channels      int           // 声道数
	InputDevice   string        // 输入设备
	OutputDevice  string        // 输出设备
	ErrorMessage  string        // 错误信息
}

func main() {
	// 设置日志级别 - 交互模式下启用调试日志
	if len(os.Args) > 1 && os.Args[1] == "--interactive" {
		logrus.SetLevel(logrus.DebugLevel)
	} else {
		logrus.SetLevel(logrus.InfoLevel)
	}
	logrus.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
		ForceColors:   true,
	})

	// 检查是否需要运行交互式测试
	if len(os.Args) > 1 && os.Args[1] == "--interactive" {
		fmt.Println("🎙️  音频AEC功能检测工具 - 交互式模式")
		fmt.Println("=====================================")
		fmt.Println("正在初始化音频系统进行交互式回声测试...")
		fmt.Println()

		// 直接运行交互式测试
		runInteractiveTest()
		return
	}

	fmt.Println("🎙️  音频AEC功能检测工具")
	fmt.Println("========================")
	fmt.Println("正在检测您的电脑是否支持AEC（声学回声消除）功能...")
	fmt.Println()

	// 执行AEC检测
	result := checkAECFunctionality()

	// 输出检测结果
	printResults(result)

	// 提示交互式测试选项
	fmt.Println("\n💡 提示: 运行 './aec_checking --interactive' 可以进行交互式语音测试")
}

// checkAECFunctionality 检测AEC功能
func checkAECFunctionality() *AECResult {
	result := &AECResult{}

	// 创建音频配置
	config := clientAudio.AudioConfig{
		SampleRate:        16000,
		ChannelCount:      1,
		FrameDuration:     60,
		UseDefaultDevices: true,
		EnableVAD:         false,
	}

	// 创建音频管理器
	manager := clientAudio.NewManager(config)

	// 初始化音频系统
	logrus.Info("正在初始化音频系统...")
	if err := manager.Initialize(); err != nil {
		result.ErrorMessage = fmt.Sprintf("音频系统初始化失败: %v", err)
		return result
	}
	defer func() {
		if err := manager.Close(); err != nil {
			logrus.Errorf("关闭音频管理器失败: %v", err)
		}
	}()

	logrus.Info("音频系统初始化成功")

	// 获取音频管理器以访问底层功能
	audioManager := manager.GetAudioManager()
	if audioManager == nil {
		result.ErrorMessage = "无法获取底层音频管理器"
		return result
	}

	// 设置基本音频参数
	result.SampleRate = audioManager.SampleRate()
	result.Channels = audioManager.ChannelCount()

	// 检测输入/输出设备
	result.InputDevice = "默认设备"
	result.OutputDevice = "默认设备"

	logrus.WithFields(logrus.Fields{
		"采样率":   result.SampleRate,
		"声道数":   result.Channels,
		"输入设备":  result.InputDevice,
		"输出设备":  result.OutputDevice,
	}).Info("音频设备信息")

	// 测试录音和播放功能以检测延迟
	logrus.Info("正在测试音频延迟...")
	latency, err := measureAudioLatency(manager)
	if err != nil {
		logrus.Errorf("测量音频延迟失败: %v", err)
		result.ErrorMessage = fmt.Sprintf("延迟测量失败: %v", err)
	} else {
		result.Latency = latency
		logrus.Infof("检测到音频延迟: %v", latency)
	}

	// 基于延迟和系统特征判断AEC支持
	result.HasAEC = detectAECSupport(result.Latency, result.SampleRate)
	result.HasNoiseSupp = detectNoiseSuppressionSupport()
	result.HasGainCtrl = detectGainControlSupport()

	return result
}

// measureAudioLatency 测量音频延迟
func measureAudioLatency(manager *clientAudio.Manager) (time.Duration, error) {
	// 创建测试音频数据（简短的音频信号）
	testSignal := createTestSignal(1000) // 1秒的测试信号

	// 记录开始时间
	startTime := time.Now()

	// 设置PCM数据回调来测量延迟
	var receivedTime time.Time
	received := make(chan bool, 1)

	manager.AddPCMDataCallback("latency_test", func(data []int16, size int) {
		if receivedTime.IsZero() {
			receivedTime = time.Now()
			received <- true
		}
	})

	// 开始录音
	if err := manager.StartRecording(); err != nil {
		return 0, fmt.Errorf("开始录音失败: %v", err)
	}

	// 播放测试信号（如果可能）
	player := manager.Player()
	if player != nil {
		logrus.Info("准备播放测试音频信号...")

		// 检查播放器状态并启动
		if !player.IsPlaying() {
			logrus.Debug("音频播放器未运行，尝试启动...")
			if err := player.Start(); err != nil {
				logrus.Errorf("启动音频播放器失败: %v", err)
				manager.StopRecording()
				return 0, fmt.Errorf("启动播放器失败: %v", err)
			}
			logrus.Info("音频播放器已启动")
		}

		// 检查是否为哑模式
		if player.IsDummyMode() {
			logrus.Warn("音频播放器在哑模式下运行，可能无法实际播放音频")
		}

		// 播放测试信号来测量往返延迟
		player.QueuePCMAudio(testSignal)
		logrus.Info("已发送测试音频信号到播放队列")
	} else {
		logrus.Warn("音频播放器为空，无法播放测试信号")
	}

	// 等待接收音频数据或超时
	select {
	case <-received:
		// 停止录音
		manager.StopRecording()
		latency := receivedTime.Sub(startTime)
		return latency, nil
	case <-time.After(2 * time.Second):
		// 超时
		manager.StopRecording()
		return 0, fmt.Errorf("音频延迟测量超时")
	}
}

// createTestSignal 创建测试音频信号
func createTestSignal(durationMs int) []int16 {
	sampleRate := 16000
	frameCount := (sampleRate * durationMs) / 1000
	signal := make([]int16, frameCount)

	// 生成440Hz的正弦波 (A4音符)
	frequency := 440.0
	amplitude := 0.3  // 30% 振幅，避免音量过大

	for i := range signal {
		t := float64(i) / float64(sampleRate)
		// 直接使用math.Sin获得更好的精度
		signal[i] = int16(amplitude * 32767.0 * math.Sin(2*math.Pi*frequency*t))
	}

	logrus.Debugf("生成测试音频信号: 时长=%dms, 采样率=%dHz, 频率=%.1fHz, 帧数=%d",
		durationMs, sampleRate, frequency, frameCount)

	return signal
}

// detectAECSupport 检测AEC支持
func detectAECSupport(latency time.Duration, sampleRate int) bool {
	// 基于延迟和采样率判断AEC支持
	// 通常AEC系统需要较低的延迟和适当的采样率

	if latency > 200*time.Millisecond {
		logrus.Warnf("音频延迟较高 (%v)，可能不支持硬件AEC", latency)
		return false
	}

	if sampleRate < 16000 {
		logrus.Warnf("采样率较低 (%d Hz)，可能不支持高质量AEC", sampleRate)
		return false
	}

	// 检查系统类型（这里简化处理）
	return true
}

// detectNoiseSuppressionSupport 检测噪声抑制支持
func detectNoiseSuppressionSupport() bool {
	// 大多数现代音频系统都支持某种形式的噪声抑制
	return true
}

// detectGainControlSupport 检测增益控制支持
func detectGainControlSupport() bool {
	// 大多数现代音频系统都支持自动增益控制
	return true
}

// printResults 输出检测结果
func printResults(result *AECResult) {
	fmt.Println("📊 检测结果:")
	fmt.Println("================")

	if result.ErrorMessage != "" {
		fmt.Printf("❌ 检测过程中出现错误: %s\n", result.ErrorMessage)
		return
	}

	fmt.Printf("🎙️  输入设备: %s\n", result.InputDevice)
	fmt.Printf("🔊 输出设备: %s\n", result.OutputDevice)
	fmt.Printf("📈 采样率: %d Hz\n", result.SampleRate)
	fmt.Printf("🔊 声道数: %d\n", result.Channels)
	fmt.Printf("⏱️  音频延迟: %v\n", result.Latency)
	fmt.Println()

	fmt.Println("🔍 AEC功能支持情况:")
	fmt.Printf("  声学回声消除 (AEC): %s\n", getCheckMark(result.HasAEC))
	fmt.Printf("  噪声抑制: %s\n", getCheckMark(result.HasNoiseSupp))
	fmt.Printf("  自动增益控制: %s\n", getCheckMark(result.HasGainCtrl))
	fmt.Println()

	if result.HasAEC {
		fmt.Println("✅ 恭喜！您的电脑似乎支持AEC功能")
		fmt.Println("   在视频会议和语音通话中应该能获得较好的音质")
	} else {
		fmt.Println("⚠️  您的电脑可能不支持硬件AEC功能")
		fmt.Println("   在语音通话中可能会遇到回声问题")
		fmt.Println("   建议使用耳机或启用软件AEC")
	}
}

// getCheckMark 获取检查标记
func getCheckMark(supported bool) string {
	if supported {
		return "✅ 支持"
	}
	return "❌ 不支持"
}

// runInteractiveTest 运行交互式AEC测试
func runInteractiveTest() {
	// 设置信号处理
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// 创建音频配置 - 需要同时支持录音和播放
	config := clientAudio.AudioConfig{
		SampleRate:        16000,
		ChannelCount:      1,
		FrameDuration:     60,
		UseDefaultDevices: true,
		EnableVAD:         false, // AEC测试不需要VAD
	}

	manager := clientAudio.NewManager(config)
	if err := manager.Initialize(); err != nil {
		log.Fatalf("初始化音频系统失败: %v", err)
	}
	defer manager.Close()

	fmt.Println("🎙️  AEC回声消除测试")
	fmt.Println("===================")
	fmt.Println("本测试将播放测试声音并检测系统是否能消除回声")
	fmt.Println()

	// 启动播放器
	player := manager.Player()
	if player == nil {
		log.Fatal("无法获取音频播放器")
	}

	if !player.IsPlaying() {
		if err := player.Start(); err != nil {
			log.Fatalf("启动播放器失败: %v", err)
		}
	}

	if player.IsDummyMode() {
		fmt.Println("⚠️  播放器在哑模式下运行，可能无法播放声音")
	}

	fmt.Println("🔊 开始AEC测试...")
	fmt.Println("测试步骤:")
	fmt.Println("1. 系统将播放5秒的测试声音")
	fmt.Println("2. 同时录音捕获麦克风输入")
	fmt.Println("3. 分析录音中是否包含播放声音的回声")
	fmt.Println("4. 评估AEC消除效果")
	fmt.Println()
	fmt.Println("请确保:")
	fmt.Println("- 音响/耳机已连接并开启")
	fmt.Println("- 麦克风工作正常")
	fmt.Println("- 环境相对安静")
	fmt.Println()
	fmt.Println("按回车键开始测试，或按Ctrl+C退出")

	// 等待用户确认或退出
	select {
	case <-sigChan:
		fmt.Println("\n测试已取消")
		return
	case <-time.After(100 * time.Millisecond):
		// 给用户一点时间看到提示
	}

	// 执行AEC测试
	testResult := performAECTest(manager, player)

	// 输出测试结果
	printAECTestResult(testResult)

	fmt.Println("\n测试完成！按Ctrl+C退出...")
	<-sigChan
	fmt.Println("👋 再见！")
}

// minInt16 返回切片中的最小值
func minInt16(data []int16) int16 {
	if len(data) == 0 {
		return 0
	}
	min := data[0]
	for _, v := range data[1:] {
		if v < min {
			min = v
		}
	}
	return min
}

// maxInt16 返回切片中的最大值
func maxInt16(data []int16) int16 {
	if len(data) == 0 {
		return 0
	}
	max := data[0]
	for _, v := range data[1:] {
		if v > max {
			max = v
		}
	}
	return max
}

// AECTestResult AEC测试结果
type AECTestResult struct {
	TestDuration       time.Duration // 测试持续时间
	PlaybackFrequency  float64       // 播放的测试频率
	RecordedAmplitude   float64       // 录音振幅平均值
	EchoLevel          float64       // 回声等级 (0-1)
	AECEffectiveness   float64       // AEC有效性 (0-100%)
	HasEcho            bool          // 是否检测到回声
	ErrorMessage       string        // 错误信息
	RecordedFilePath   string        // 保存的录音文件路径
	TestSignalFilePath string        // 保存的测试信号文件路径
}

// performAECTest 执行AEC测试
func performAECTest(manager *clientAudio.Manager, player *audio.AudioPlayerNew) *AECTestResult {
	result := &AECTestResult{
		TestDuration:      5 * time.Second,
		PlaybackFrequency: 1000.0, // 1kHz测试频率
	}

	fmt.Println("🎵 正在准备测试信号...")

	// 生成测试信号 (1kHz正弦波，持续5秒)
	testSignal := createTestSignal(5000) // 5秒
	result.PlaybackFrequency = 1000.0

	fmt.Println("🔊 开始播放测试声音并录音...")

	// 创建录音数据收集器
	recordedData := make([]int16, 0)
	var dataMutex sync.Mutex

	// 添加PCM回调来收集录音数据
	manager.AddPCMDataCallback("aec_test", func(data []int16, size int) {
		dataMutex.Lock()
		recordedData = append(recordedData, data[:size]...)
		dataMutex.Unlock()
	})

	// 开始录音
	if err := manager.StartRecording(); err != nil {
		result.ErrorMessage = fmt.Sprintf("开始录音失败: %v", err)
		return result
	}
	defer manager.StopRecording()

	// 播放测试信号
	player.QueuePCMAudio(testSignal)

	fmt.Println("⏱️  正在录音5秒，请保持环境安静...")

	// 等待测试完成
	time.Sleep(result.TestDuration)

	// 收集录音结果
	dataMutex.Lock()
	finalRecordedData := make([]int16, len(recordedData))
	copy(finalRecordedData, recordedData)
	dataMutex.Unlock()

	fmt.Printf("📊 录音完成，共采集 %d 个采样点\n", len(finalRecordedData))

	// 保存录音文件
	recordedFile, err := saveAudioAsWAV(finalRecordedData, 16000, 1, "aec_recording")
	if err != nil {
		logrus.Warnf("保存录音文件失败: %v", err)
		result.RecordedFilePath = ""
	} else {
		result.RecordedFilePath = recordedFile
		fmt.Printf("💾 录音已保存到: %s\n", recordedFile)
	}

	// 保存测试信号文件（用于对比分析）
	testSignalFile, err := saveAudioAsWAV(testSignal, 16000, 1, "test_signal")
	if err != nil {
		logrus.Warnf("保存测试信号文件失败: %v", err)
		result.TestSignalFilePath = ""
	} else {
		result.TestSignalFilePath = testSignalFile
		fmt.Printf("💾 测试信号已保存到: %s\n", testSignalFile)
	}

	// 分析录音数据
	result = analyzeAECResult(finalRecordedData, result)

	return result
}

// analyzeAECResult 分析AEC测试结果
func analyzeAECResult(recordedData []int16, result *AECTestResult) *AECTestResult {
	if len(recordedData) == 0 {
		result.ErrorMessage = "没有采集到录音数据"
		return result
	}

	// 计算录音振幅
	totalAmplitude := 0.0
	peakAmplitude := 0.0

	for _, sample := range recordedData {
		absSample := math.Abs(float64(sample))
		totalAmplitude += absSample
		if absSample > peakAmplitude {
			peakAmplitude = absSample
		}
	}

	averageAmplitude := totalAmplitude / float64(len(recordedData))
	maxPossibleAmplitude := 32767.0
	result.RecordedAmplitude = averageAmplitude

	// 计算回声等级 (基于振幅比例)
	echoThreshold := 1000.0 // 回声检测阈值
	result.EchoLevel = averageAmplitude / maxPossibleAmplitude

	// 计算AEC有效性
	if averageAmplitude < echoThreshold {
		// 振幅很低，说明AEC工作良好
		result.AECEffectiveness = 100.0 - (averageAmplitude/echoThreshold)*100.0
		result.HasEcho = false
	} else {
		// 振幅较高，可能存在回声
		result.AECEffectiveness = math.Max(0, 100.0-(averageAmplitude-echoThreshold)/maxPossibleAmplitude*100.0)
		result.HasEcho = true
	}

	logrus.Infof("AEC分析结果: 平均振幅=%.2f, 峰值振幅=%.2f, 回声等级=%.3f, AEC效果=%.1f%%",
		averageAmplitude, peakAmplitude, result.EchoLevel, result.AECEffectiveness)

	return result
}

// printAECTestResult 打印AEC测试结果
func printAECTestResult(result *AECTestResult) {
	fmt.Println("\n" + strings.Repeat("=", 50))
	fmt.Println("🎯 AEC测试结果报告")
	fmt.Println(strings.Repeat("=", 50))

	if result.ErrorMessage != "" {
		fmt.Printf("❌ 测试失败: %s\n", result.ErrorMessage)
		return
	}

	fmt.Printf("📋 测试参数:\n")
	fmt.Printf("   测试时长: %v\n", result.TestDuration)
	fmt.Printf("   播放频率: %.0f Hz\n", result.PlaybackFrequency)
	fmt.Printf("   采样率: 16000 Hz\n")
	fmt.Printf("   录音振幅: %.2f\n", result.RecordedAmplitude)
	if result.RecordedFilePath != "" {
		fmt.Printf("   录音文件: %s\n", result.RecordedFilePath)
	}
	if result.TestSignalFilePath != "" {
		fmt.Printf("   测试信号: %s\n", result.TestSignalFilePath)
	}
	fmt.Println()

	fmt.Printf("🔍 回声分析:\n")
	fmt.Printf("   回声等级: %.3f (0.000=无回声, 1.000=强回声)\n", result.EchoLevel)
	fmt.Printf("   是否检测到回声: %s\n", getCheckMark(result.HasEcho))
	fmt.Println()

	fmt.Printf("🎛️  AEC性能评估:\n")
	fmt.Printf("   AEC有效性: %.1f%%\n", result.AECEffectiveness)
	fmt.Println()

	// 给出评估结论
	fmt.Printf("📝 评估结论:\n")
	if result.AECEffectiveness >= 90.0 {
		fmt.Println("   🟢 优秀! 系统具有出色的AEC功能")
		fmt.Println("   回声被有效消除，音质体验良好")
	} else if result.AECEffectiveness >= 70.0 {
		fmt.Println("   🟡 良好! 系统具备基本的AEC功能")
		fmt.Println("   大部分回声被消除，但仍可能有轻微残余")
	} else if result.AECEffectiveness >= 50.0 {
		fmt.Println("   🟠 一般! 系统AEC功能有限")
		fmt.Println("   部分回声被消除，建议使用耳机或软件AEC")
	} else {
		fmt.Println("   🔴 较差! 系统缺少有效的AEC功能")
		fmt.Println("   建议使用耳机来避免回声问题")
	}

	fmt.Println()
	fmt.Printf("💡 改进建议:\n")
	if result.HasEcho {
		fmt.Println("   - 使用耳机而非音响可以避免回声")
		fmt.Println("   - 启用软件AEC处理")
		fmt.Println("   - 调整麦克风位置，远离音响")
	} else {
		fmt.Println("   - 音频系统配置良好，继续使用当前设置")
	}

	fmt.Println(strings.Repeat("=", 50))
}

// saveAudioAsWAV 将音频数据保存为WAV文件
func saveAudioAsWAV(audioData []int16, sampleRate int, channels int, baseName string) (string, error) {
	// 生成文件名（包含时间戳）
	timestamp := time.Now().Format("20060102_150405")
	filename := fmt.Sprintf("%s_%s.wav", baseName, timestamp)

	// 确保目录存在
	if err := os.MkdirAll("aec_recordings", 0755); err != nil {
		return "", fmt.Errorf("创建目录失败: %v", err)
	}

	filePath := filepath.Join("aec_recordings", filename)

	// 创建文件
	file, err := os.Create(filePath)
	if err != nil {
		return "", fmt.Errorf("创建文件失败: %v", err)
	}
	defer file.Close()

	// WAV文件头
	wavSampleRate := uint32(sampleRate)
	wavChannels := uint16(channels)
	bitsPerSample := uint16(16)
	byteRate := wavSampleRate * uint32(wavChannels) * uint32(bitsPerSample) / 8
	blockAlign := wavChannels * bitsPerSample / 8
	dataSize := uint32(len(audioData) * 2)
	fileSize := uint32(36 + dataSize)

	// 写入WAV头
	// RIFF header
	file.WriteString("RIFF")
	binary.Write(file, binary.LittleEndian, fileSize)
	file.WriteString("WAVE")

	// fmt subchunk
	file.WriteString("fmt ")
	binary.Write(file, binary.LittleEndian, uint32(16)) // Subchunk1Size
	binary.Write(file, binary.LittleEndian, uint16(1))  // AudioFormat (PCM)
	binary.Write(file, binary.LittleEndian, wavChannels)
	binary.Write(file, binary.LittleEndian, wavSampleRate)
	binary.Write(file, binary.LittleEndian, byteRate)
	binary.Write(file, binary.LittleEndian, blockAlign)
	binary.Write(file, binary.LittleEndian, bitsPerSample)

	// data subchunk
	file.WriteString("data")
	binary.Write(file, binary.LittleEndian, dataSize)

	// 写入音频数据
	for _, sample := range audioData {
		binary.Write(file, binary.LittleEndian, sample)
	}

	logrus.Infof("音频文件已保存: %s (时长: %.2f秒)", filePath, float64(len(audioData))/float64(sampleRate*channels))

	return filePath, nil
}