# 项目名称和版本
APP_NAME := xiaozhi-client
VERSION := 1.0.0
BOARD_TYPE := generic

# Go相关变量
GO := go
GO_BUILD := $(GO) build
GO_CLEAN := $(GO) clean
GO_TEST := $(GO) test
GO_MOD := $(GO) mod
GOFLAGS :=
LDFLAGS := -ldflags "-X main.Version=$(VERSION)"

# 主目标文件
MAIN_PKG := ./cmd/client
TARGET := $(APP_NAME)

# 默认目标
.PHONY: all
all: build

# 显示帮助信息
.PHONY: help
help:
	@echo "小智WebSocket客户端 构建系统"
	@echo
	@echo "使用方法:"
	@echo "  make [target]"
	@echo
	@echo "支持的目标:"
	@echo "  build          - 编译程序"
	@echo "  run            - 编译并运行程序"
	@echo "  clean          - 清理编译产物"
	@echo "  test           - 运行测试"
	@echo "  deps           - 下载依赖"
	@echo "  activate       - 编译并执行激活流程"
	@echo "  help           - 显示此帮助信息"
	@echo
	@echo "环境变量:"
	@echo "  VERSION        - 设置版本号 (默认: $(VERSION))"
	@echo "  BOARD_TYPE     - 设置设备板型号 (默认: $(BOARD_TYPE))"
	@echo "  SERVER_URL     - 设置WebSocket服务器地址"
	@echo "  TOKEN          - 设置API访问令牌"
	@echo
	@echo "示例:"
	@echo "  make build VERSION=1.1.0"
	@echo "  make run SERVER_URL=wss://your-server.com"
	@echo "  make activate BOARD_TYPE=custom-board"

# 构建目标
.PHONY: build
build:
	@echo "编译 $(APP_NAME) 版本 $(VERSION)..."
	$(GO_BUILD) $(GOFLAGS) $(LDFLAGS) -o $(TARGET) $(MAIN_PKG)
	@echo "编译完成: $(TARGET)"

# 运行目标
.PHONY: run
run: build
	@echo "运行 $(APP_NAME)..."
	@RUN_ARGS="-version $(VERSION) -board $(BOARD_TYPE)"; \
	if [ ! -z "$(SERVER_URL)" ]; then \
		RUN_ARGS="$$RUN_ARGS -server $(SERVER_URL)"; \
	fi; \
	if [ ! -z "$(TOKEN)" ]; then \
		RUN_ARGS="$$RUN_ARGS -token $(TOKEN)"; \
	fi; \
	echo "执行: ./$(TARGET) $$RUN_ARGS"; \
	./$(TARGET) $$RUN_ARGS

# 激活目标
.PHONY: activate
activate: build
	@echo "执行激活流程..."
	@RUN_ARGS="-version $(VERSION) -board $(BOARD_TYPE) -activate-only"; \
	if [ ! -z "$(SERVER_URL)" ]; then \
		RUN_ARGS="$$RUN_ARGS -server $(SERVER_URL)"; \
	fi; \
	if [ ! -z "$(TOKEN)" ]; then \
		RUN_ARGS="$$RUN_ARGS -token $(TOKEN)"; \
	fi; \
	echo "执行: ./$(TARGET) $$RUN_ARGS"; \
	./$(TARGET) $$RUN_ARGS

# 清理目标
.PHONY: clean
clean:
	@echo "清理..."
	$(GO_CLEAN)
	rm -f $(TARGET)
	@echo "清理完成"

# 下载依赖
.PHONY: deps
deps:
	@echo "下载依赖..."
	$(GO_MOD) tidy
	@echo "依赖下载完成"

# 测试目标
.PHONY: test
test:
	@echo "运行测试..."
	$(GO_TEST) ./...
	@echo "测试完成" 