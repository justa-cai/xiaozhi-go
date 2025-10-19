# 小智(Xiaozhi)智能语音助手客户端

[![Go版本](https://img.shields.io/badge/Go-1.20+-00ADD8?style=flat&logo=go)](https://golang.org/)
[![许可证](https://img.shields.io/badge/许可证-MIT-green)](LICENSE)
[![构建状态](https://img.shields.io/github/actions/workflow/status/JustaCai/xiaozhi-go/build.yml?branch=main)](https://github.com/justa-cai/xiaozhi-go/actions/workflows/build.yml)

小智(Xiaozhi)是一个基于WebSocket协议的智能语音助手客户端，支持实时语音识别、对话和物联网控制功能。

## 功能特点

- 💬 **实时语音交互**：支持Opus音频编解码，实现低延迟的语音沟通
- 🔊 **语音识别和合成**：集成语音转文本(STT)和文本转语音(TTS)功能
- 🏠 **IoT设备控制**：支持通过语音指令控制物联网设备
- 🌐 **WebSocket协议**：基于标准WebSocket实现稳定可靠的通信
- 🔄 **自动重连机制**：网络异常时自动重新连接到服务器
- 🔒 **安全认证**：支持令牌认证，确保通信安全

## 快速开始

### 前置条件

- Go 1.20+
- 支持PortAudio的系统环境
- 有效的服务端接口和认证令牌

### 安装

#### 方法1：下载预编译版本

您可以从[GitHub Releases](https://github.com/justa-cai/xiaozhi-go/releases)页面下载适合您操作系统的预编译版本。

#### 方法2：从源码构建

1. 克隆代码库：

```bash
git clone https://github.com/justa-cai/xiaozhi-go.git
cd xiaozhi-go
```

2. 安装依赖：

```bash
make deps
```

3. 编译项目：

```bash
make build
```

### 使用方法

#### 构建和运行主应用

**使用 build.sh 脚本（推荐）**：

```bash
# 编译并运行
./build.sh --run --server wss://your-server.com --token your-token

# 仅编译
./build.sh --build

# 清理编译产物
./build.sh --clean

# 仅执行设备激活
./build.sh --activate --server wss://your-server.com --token your-token

# 指定版本和板型号
./build.sh --run --version 1.0.1 --board rpi4 --server wss://your-server.com --token your-token
```

**使用 Makefile**：

```bash
# 编译并运行主应用
make run SERVER_URL=wss://your-server.com TOKEN=your-token

# 仅编译
make build

# 清理编译产物
make clean

# 安装依赖
make deps
```

**直接运行可执行文件**：

```bash
./xiaozhi-client -server wss://your-server.com -token your-token
```

#### 工具应用

**唤醒词检测工具**：

```bash
# 使用 Makefile 编译并运行
make wake-word-run

# 或直接运行
./bin/wake-word -verbose -audio-debug -fast-speech
```

唤醒词检测工具参数：
- `-verbose`: 启用详细调试输出
- `-audio-debug`: 启用电平调试
- `-show-all`: 显示所有识别文本（不仅是关键词）
- `-fast-speech`: 启用快速语音检测优化
- `-model-path`: 模型目录路径
- `-keywords`: 关键词文件路径

**语音活动检测（VAD）工具**：

```bash
go run ./cmd/vad/ -model ./models/silero_vad.onnx -save-audio -output-dir ./speech_segments
```

VAD工具参数：
- `-model`: VAD模型路径（silero_vad.onnx 或 ten-vad.onnx）
- `-threshold`: VAD阈值（默认：0.5）
- `-min-silence`: 最小静音时长（默认：0.5秒）
- `-min-speech`: 最小语音时长（默认：0.25秒）
- `-max-speech`: 最大语音时长（默认：10秒）
- `-save-audio`: 启用语音片段保存为WAV文件
- `-output-dir`: 语音片段输出目录

### 命令行参数

#### 主应用参数

| 参数 | 描述 | 默认值 |
|------|------|--------|
| `-server` | WebSocket服务器地址 | wss://api.tenclass.net/xiaozhi/v1/ |
| `-token` | API访问令牌 | - |
| `-version` | 客户端版本号 | 1.0.0 |
| `-board` | 设备板型号 | generic |
| `-activate-only` | 仅执行激活流程 | false |

#### 构建脚本参数（build.sh）

| 参数 | 描述 |
|------|------|
| `-h, --help` | 显示帮助信息 |
| `-b, --build` | 只编译程序 |
| `-r, --run` | 编译并运行程序 |
| `-c, --clean` | 清理编译产物 |
| `-a, --activate` | 只执行设备激活 |
| `-v, --version` | 指定版本号 |
| `-t, --board-type` | 指定设备板型号 |
| `--server` | 指定WebSocket服务器地址 |
| `--token` | 指定API访问令牌 |

## 自动构建

本项目使用GitHub Actions进行持续集成和自动构建。每当代码推送到主分支或创建新标签时，都会自动触发构建流程，为Windows、macOS和Linux平台生成可执行文件。

### 发布流程

1. 创建新版本标签（例如`v1.0.1`）
2. 推送标签到GitHub
3. GitHub Actions将自动构建所有平台版本
4. 构建完成后，发布包将自动上传到GitHub Releases页面

### 手动触发构建

您也可以在GitHub仓库的Actions页面手动触发构建流程：

1. 导航到仓库的"Actions"标签页
2. 选择"构建跨平台应用"工作流
3. 点击"Run workflow"按钮
4. 选择分支并确认启动构建

## 项目结构

```
xiaozhi-go/
├── cmd/                   # 命令行应用
│   ├── client/           # 主客户端应用
│   │   ├── main.go       # 应用入口
│   │   ├── core/         # 核心应用逻辑
│   │   ├── audio/        # 音频处理模块
│   │   ├── network/      # 网络通信模块
│   │   ├── interaction/  # 交互模块
│   │   ├── device/       # 设备管理模块
│   │   ├── config/       # 配置管理
│   │   └── utils/        # 工具函数
│   ├── wake-word/        # 唤醒词检测工具
│   ├── vad/              # 语音活动检测工具
│   └── audio_demos/      # 音频相关示例
├── internal/              # 内部包
│   ├── audio/            # 音频处理
│   ├── protocol/         # 通信协议实现
│   ├── iot/              # 物联网功能
│   ├── client/           # 客户端核心逻辑
│   └── ota/              # 在线更新功能
├── models/                # 模型文件目录
│   └── sherpa-onnx-.../  # 唤醒词检测模型
├── doc/                   # 文档
│   └── websocket.md      # WebSocket协议文档
├── build.sh               # 高级构建脚本
├── Makefile               # 简易构建工具
└── go.mod                 # Go模块定义
```

### 构建系统说明

本项目提供两种构建方式：

**build.sh（推荐）**：
- 功能完整的构建脚本，支持彩色输出、依赖检查、多种运行模式
- 提供完整的命令行参数支持
- 包含错误处理和详细的帮助信息

**Makefile**：
- 简洁的构建工具，适合快速编译和测试
- 支持基本的构建、运行、清理操作
- 与标准Go开发流程兼容

## 协议文档

有关WebSocket通信协议的详细信息，请参阅[WebSocket协议文档](doc/websocket.md)。

## 开发

### 编译与测试

```bash
# 编译项目
make build

# 运行测试
make test

# 清理编译产物
make clean
```

### 环境变量

您也可以通过环境变量配置客户端：

- `VERSION` - 客户端版本号
- `BOARD_TYPE` - 设备板型号
- `SERVER_URL` - WebSocket服务器地址
- `TOKEN` - API访问令牌
- `ACTIVATE_ONLY` - 是否仅执行激活流程

## 许可证

本项目采用MIT许可证，详情请参阅[LICENSE](LICENSE)文件。

## 贡献

欢迎提交问题和贡献代码！请遵循以下步骤：

1. Fork本仓库
2. 创建功能分支 (`git checkout -b feature/amazing-feature`)
3. 提交更改 (`git commit -m 'Add some amazing feature'`)
4. 推送到分支 (`git push origin feature/amazing-feature`)
5. 打开Pull Request
