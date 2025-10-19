# Xiaozhi (小智) Intelligent Voice Assistant Client

[![Go Version](https://img.shields.io/badge/Go-1.20+-00ADD8?style=flat&logo=go)](https://golang.org/)
[![License](https://img.shields.io/badge/License-MIT-green)](LICENSE)
[![Build Status](https://img.shields.io/github/actions/workflow/status/JustaCai/xiaozhi-go/build.yml?branch=main)](https://github.com/justa-cai/xiaozhi-go/actions/workflows/build.yml)

Xiaozhi is a WebSocket-based intelligent voice assistant client that supports real-time voice recognition, conversation, and IoT device control functionality.

## Features

- 💬 **Real-time Voice Interaction**: Opus audio codec support for low-latency voice communication
- 🔊 **Speech Recognition and Synthesis**: Integrated STT (Speech-to-Text) and TTS (Text-to-Speech) functionality
- 🏠 **IoT Device Control**: Control IoT devices through voice commands
- 🌐 **WebSocket Protocol**: Stable and reliable communication based on standard WebSocket
- 🔄 **Auto-reconnection**: Automatically reconnect to server on network exceptions
- 🔒 **Secure Authentication**: Token-based authentication for secure communication

## Quick Start

### Prerequisites

- Go 1.20+
- PortAudio-compatible system environment
- Valid server endpoint and authentication token

### Installation

#### Method 1: Download Pre-compiled Version

You can download pre-compiled versions for your operating system from the [GitHub Releases](https://github.com/justa-cai/xiaozhi-go/releases) page.

#### Method 2: Build from Source

1. Clone the repository:

```bash
git clone https://github.com/justa-cai/xiaozhi-go.git
cd xiaozhi-go
```

2. Install dependencies:

```bash
make deps
```

3. Build the project:

```bash
make build
```

### Usage

#### Building and Running the Main Application

**Using build.sh Script (Recommended)**:

```bash
# Build and run
./build.sh --run --server wss://your-server.com --token your-token

# Build only
./build.sh --build

# Clean build artifacts
./build.sh --clean

# Device activation only
./build.sh --activate --server wss://your-server.com --token your-token

# Specify version and board type
./build.sh --run --version 1.0.1 --board rpi4 --server wss://your-server.com --token your-token
```

**Using Makefile**:

```bash
# Build and run main application
make run SERVER_URL=wss://your-server.com TOKEN=your-token

# Build only
make build

# Clean build artifacts
make clean

# Install dependencies
make deps
```

**Direct Execution**:

```bash
./xiaozhi-client -server wss://your-server.com -token your-token
```

#### Utility Applications

**Wake Word Detection Tool**:

```bash
# Build and run using Makefile
make wake-word-run

# Or run directly
./bin/wake-word -verbose -audio-debug -fast-speech
```

Wake word detection tool parameters:
- `-verbose`: Enable verbose debug output
- `-audio-debug`: Enable audio level debugging
- `-show-all`: Show all recognized text (not just keywords)
- `-fast-speech`: Enable fast speech detection optimizations
- `-model-path`: Model directory path
- `-keywords`: Keywords file path

**Voice Activity Detection (VAD) Tool**:

```bash
go run ./cmd/vad/ -model ./models/silero_vad.onnx -save-audio -output-dir ./speech_segments
```

VAD tool parameters:
- `-model`: VAD model path (silero_vad.onnx or ten-vad.onnx)
- `-threshold`: VAD threshold (default: 0.5)
- `-min-silence`: Minimum silence duration in seconds (default: 0.5s)
- `-min-speech`: Minimum speech duration in seconds (default: 0.25s)
- `-max-speech`: Maximum speech duration in seconds (default: 10s)
- `-save-audio`: Enable saving speech segments as WAV files
- `-output-dir`: Speech segment output directory

### Command Line Parameters

#### Main Application Parameters

| Parameter | Description | Default |
|-----------|-------------|---------|
| `-server` | WebSocket server address | wss://api.tenclass.net/xiaozhi/v1/ |
| `-token` | API access token | - |
| `-version` | Client version number | 1.0.0 |
| `-board` | Device board type | generic |
| `-activate-only` | Device activation only | false |

#### Build Script Parameters (build.sh)

| Parameter | Description |
|-----------|-------------|
| `-h, --help` | Show help information |
| `-b, --build` | Build only |
| `-r, --run` | Build and run |
| `-c, --clean` | Clean build artifacts |
| `-a, --activate` | Device activation only |
| `-v, --version` | Specify version number |
| `-t, --board-type` | Specify device board type |
| `--server` | Specify WebSocket server address |
| `--token` | Specify API access token |

## Automated Builds

This project uses GitHub Actions for continuous integration and automated builds. Every time code is pushed to the main branch or a new tag is created, it automatically triggers the build process to generate executable files for Windows, macOS, and Linux platforms.

### Release Process

1. Create a new version tag (e.g., `v1.0.1`)
2. Push the tag to GitHub
3. GitHub Actions will automatically build all platform versions
4. After completion, release packages will be automatically uploaded to the GitHub Releases page

### Manual Build Trigger

You can also manually trigger the build process on the GitHub repository's Actions page:

1. Navigate to the repository's "Actions" tab
2. Select the "Build Cross-Platform Applications" workflow
3. Click the "Run workflow" button
4. Select branch and confirm to start the build

## Project Structure

```
xiaozhi-go/
├── cmd/                   # Command-line applications
│   ├── client/           # Main client application
│   │   ├── main.go       # Application entry point
│   │   ├── core/         # Core application logic
│   │   ├── audio/        # Audio processing module
│   │   ├── network/      # Network communication module
│   │   ├── interaction/  # Interaction module
│   │   ├── device/       # Device management module
│   │   ├── config/       # Configuration management
│   │   └── utils/        # Utility functions
│   ├── wake-word/        # Wake word detection tool
│   ├── vad/              # Voice activity detection tool
│   └── audio_demos/      # Audio-related examples
├── internal/              # Internal packages
│   ├── audio/            # Audio processing
│   ├── protocol/         # Communication protocol implementation
│   ├── iot/              # IoT functionality
│   ├── client/           # Client core logic
│   └── ota/              # Over-the-air update functionality
├── models/                # Model files directory
│   └── sherpa-onnx-.../  # Wake word detection models
├── doc/                   # Documentation
│   └── websocket.md      # WebSocket protocol documentation
├── build.sh               # Advanced build script
├── Makefile               # Simple build tool
└── go.mod                 # Go module definition
```

### Build System Description

This project provides two build methods:

**build.sh (Recommended)**:
- Feature-complete build script with colored output, dependency checking, multiple run modes
- Complete command-line parameter support
- Error handling and detailed help information

**Makefile**:
- Concise build tool suitable for quick compilation and testing
- Supports basic build, run, and clean operations
- Compatible with standard Go development workflow

## Protocol Documentation

For detailed information about the WebSocket communication protocol, please refer to the [WebSocket Protocol Documentation](doc/websocket.md).

## Development

### Build and Test

```bash
# Build project
make build

# Run tests
make test

# Clean build artifacts
make clean
```

### Environment Variables

You can also configure the client through environment variables:

- `VERSION` - Client version number
- `BOARD_TYPE` - Device board type
- `SERVER_URL` - WebSocket server address
- `TOKEN` - API access token
- `ACTIVATE_ONLY` - Whether to execute activation process only

## License

This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.

## Contributing

Welcome to submit issues and contribute code! Please follow these steps:

1. Fork this repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request