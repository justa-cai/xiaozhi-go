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

**基本运行**：

```bash
./xiaozhi-client -server wss://your-server.com -token your-token
```

**使用Makefile运行**：

```bash
make run SERVER_URL=wss://your-server.com TOKEN=your-token
```

**仅执行激活流程**：

```bash
make activate SERVER_URL=wss://your-server.com TOKEN=your-token
```

### 命令行参数

| 参数 | 描述 | 默认值 |
|------|------|--------|
| `-server` | WebSocket服务器地址 | wss://api.tenclass.net/xiaozhi/v1/ |
| `-token` | API访问令牌 | - |
| `-version` | 客户端版本号 | 1.0.0 |
| `-board` | 设备板型号 | generic |
| `-activate-only` | 仅执行激活流程 | false |

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
│   └── audio_demos/      # 音频相关示例
├── internal/              # 内部包
│   ├── audio/            # 音频处理
│   ├── protocol/         # 通信协议实现
│   ├── iot/              # 物联网功能
│   ├── client/           # 客户端核心逻辑
│   └── ota/              # 在线更新功能
├── doc/                   # 文档
│   └── websocket.md      # WebSocket协议文档
├── Makefile               # 项目构建脚本
└── go.mod                 # Go模块定义
```

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
