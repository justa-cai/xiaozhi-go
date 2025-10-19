# 小智(Xiaozhi)智能语音助手客户端 - 设计文档

## 项目概述

小智(Xiaozhi)是一个基于WebSocket协议的智能语音助手客户端，支持实时语音识别、对话和物联网控制功能。该项目使用Go语言开发，采用模块化架构设计，支持跨平台运行。

## 项目结构

```
xiaozhi-go/
├── cmd/                   # 命令行应用
│   ├── client/           # 主客户端应用
│   └── audio_demos/      # 音频相关示例
├── internal/              # 内部包
│   ├── audio/            # 音频处理
│   ├── protocol/         # 通信协议实现
│   ├── client/           # 客户端核心逻辑
│   └── ota/              # 在线更新功能
├── doc/                   # 文档
├── Makefile               # 项目构建脚本
└── go.mod                 # Go模块定义
```

## 核心架构

### 1. 主客户端应用 (cmd/client/main.go)

主客户端应用是整个系统的核心，负责协调音频处理、协议通信和用户交互。主要功能包括：

- **设备管理**: 获取设备ID、处理设备激活
- **音频管理**: 初始化录音和播放功能
- **WebSocket通信**: 连接到服务器并处理消息
- **用户交互**: 处理键盘输入（F2开始录音，S停止录音，Q退出）
- **状态管理**: 管理客户端状态（空闲、监听、说话）

#### 主要流程
1. 初始化日志和命令行参数
2. 获取设备ID（优先使用MAC地址，否则生成临时ID）
3. 如果仅执行激活流程，则跳过主流程
4. 检查设备是否已激活
5. 初始化音频系统
6. 创建WebSocket协议实例并连接服务器
7. 设置各种回调函数处理状态变更、网络错误、识别文本等
8. 进入主循环，处理键盘输入、按键事件和心跳

### 2. 音频处理系统 (internal/audio/)

音频处理系统是项目的关键组件，负责录音和播放功能。

#### 音频管理器 (AudioManagerNew)
- **录音器 (Recorder)**: 负责从麦克风捕获音频数据
- **播放器 (AudioPlayerNew)**: 负责播放从服务器接收的音频数据
- **编解码器 (OpusCodec)**: 使用Opus编码器处理音频数据

#### 平台支持
- **Linux**: 使用PulseAudio进行录音
- **macOS**: 使用CoreAudio进行录音
- **Windows**: 使用WASAPI进行录音

#### 音频参数
- 默认采样率: 16000 Hz
- 默认通道数: 1 (单声道)
- 默认帧持续时间: 60ms
- 编码格式: Opus

### 3. 通信协议层 (internal/protocol/)

通信协议层实现了与服务器的通信，基于WebSocket协议。

#### 协议接口 (Protocol)
定义了通信的基本接口：
- Connect/Disconnect: 连接管理
- SendJSON/SendBinary: 数据发送
- 回调设置: 各种事件的回调函数
- 状态查询: 连接状态检查

#### WebSocket协议实现 (WebsocketProtocol)
- **连接管理**: 处理WebSocket连接、重连机制
- **消息处理**: 处理JSON和二进制消息
- **超时控制**: 读写超时、握手超时
- **安全选项**: TLS证书验证控制

### 4. 客户端核心逻辑 (internal/client/)

客户端核心逻辑封装了与服务器交互的业务逻辑。

#### 状态管理
- StateIdle: 空闲状态
- StateConnecting: 正在连接状态
- StateListening: 监听状态（录音中）
- StateSpeaking: 播放状态（播放TTS）

#### 监听模式
- ListenModeAuto: 自动模式
- ListenModeManual: 手动模式
- ListenModeRealtime: 实时模式

#### 主要功能
- OpenAudioChannel/CloseAudioChannel: 音频通道管理
- SendStartListening/SendStopListening: 控制录音
- SendAudioData: 发送音频数据
- 各种消息处理（STT、TTS、LLM、IoT）

### 5. OTA更新机制 (internal/ota/)

OTA（Over-The-Air）更新机制负责设备激活和固件更新检查。

#### 主要功能
- **设备激活**: 向服务器请求设备激活码
- **固件检查**: 检查是否有新版本固件
- **MQTT配置**: 获取MQTT服务器配置
- **激活状态**: 检查设备是否已激活

### 6. IoT集成

项目支持IoT设备控制功能，可以接收和处理来自服务器的IoT命令。

#### IoT消息处理
- 接收IoT命令并触发回调
- 发送IoT状态和描述符
- 与智能家居设备交互

## 技术栈

### 核心依赖
- **gorilla/websocket**: WebSocket通信
- **hajimehoshi/oto**: 音频播放
- **justa-cai/go-libopus**: Opus音频编解码
- **sirupsen/logrus**: 日志记录
- **google/uuid**: UUID生成

### 音频处理
- **Opus编解码**: 高效的音频压缩格式
- **实时处理**: 支持低延迟的音频流处理
- **跨平台**: 支持Linux、macOS、Windows的音频输入输出

## 配置参数

### 命令行参数
- `-server`: WebSocket服务器地址
- `-token`: API访问令牌
- `-version`: 客户端版本号
- `-board`: 设备板型号
- `-activate-only`: 仅执行激活流程
- `-log-level`: 日志级别
- `-skip-tls-verify`: 跳过TLS证书验证
- `-http-proxy`: HTTP代理地址
- `-debug`: 启用高级调试功能
- `-verbose`: 启用详细日志

### 默认配置
- 默认服务器URL: `wss://api.tenclass.net/xiaozhi/v1/`
- 默认采样率: 16000 Hz
- 默认音频格式: Opus
- 默认帧持续时间: 60ms

## 构建与部署

### 构建系统 (Makefile)
项目使用Makefile进行构建管理，支持以下目标：
- `make build`: 编译程序
- `make run`: 编译并运行程序
- `make clean`: 清理编译产物
- `make test`: 运行测试
- `make deps`: 下载依赖
- `make activate`: 编译并执行激活流程

### 构建环境
- Go 1.23.3+
- 支持PortAudio的系统环境
- 有效的服务端接口和认证令牌

## 安全特性

### 认证机制
- **令牌认证**: 使用Bearer令牌进行API访问认证
- **设备ID**: 基于MAC地址生成的唯一设备标识
- **客户端ID**: 基于设备ID生成的UUID客户端标识

### 通信安全
- **TLS支持**: 支持TLS加密通信
- **证书验证**: 可选择跳过TLS证书验证（用于测试环境）

## 运行时特性

### 自动重连
- 网络异常时自动重新连接到服务器
- 指数退避重连策略

### 错误处理
- 详细的错误分析和日志记录
- 资源清理和状态恢复
- 快速退出机制

### 性能优化
- 音频数据通道缓冲
- 非阻塞音频处理
- 心跳机制保持连接

## 扩展性设计

### 模块化架构
- 清晰的接口定义
- 松耦合组件设计
- 易于扩展的回调机制

### 跨平台支持
- 平台特定的音频实现
- 统一的接口抽象
- 可配置的音频参数

## 总结

小智(Xiaozhi)智能语音助手客户端是一个功能完整、架构清晰的语音交互系统。通过模块化设计，项目实现了音频处理、协议通信、状态管理等核心功能，并提供了良好的扩展性和跨平台支持。项目具有完整的错误处理、安全认证和自动重连机制，适合在生产环境中使用。