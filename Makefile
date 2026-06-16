# dbbackupctl Makefile

# 二进制文件名
BINARY_NAME=dbbackupctl

# Go 参数
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod

# 构建目录
BUILD_DIR=./bin

# 版本信息
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT=$(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
DATE=$(shell date -u '+%Y-%m-%dT%H:%M:%SZ')

# 链接参数
LDFLAGS=-ldflags "-X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)"

# 构建平台
PLATFORMS=linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64

.PHONY: all build clean test lint fmt vet tidy check-build install uninstall help

## all: 构建并测试
all: test build

## build: 构建二进制文件
build:
	@echo "正在构建 $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/dbbackupctl

## build-all: 构建所有平台产物
build-all:
	@echo "正在构建所有平台产物..."
	@mkdir -p $(BUILD_DIR)
	@for platform in $(PLATFORMS); do \
		os=$${platform%/*}; \
		arch=$${platform#*/}; \
		output=$(BUILD_DIR)/$(BINARY_NAME)-$${os}-$${arch}; \
		if [ "$$os" = "windows" ]; then output=$$output.exe; fi; \
		echo "正在构建 $$os/$$arch..."; \
		GOOS=$$os GOARCH=$$arch $(GOBUILD) $(LDFLAGS) -o $$output ./cmd/dbbackupctl; \
	done

## test: 运行测试
test:
	@echo "正在运行测试..."
	$(GOTEST) -v ./...

## lint: 运行静态检查工具
lint:
	@echo "正在运行静态检查工具..."
	golangci-lint run ./...

## fmt: 格式化代码
fmt:
	@echo "正在格式化代码..."
	$(GOCMD) fmt ./...

## vet: 运行 go vet
vet:
	@echo "正在运行 go vet..."
	$(GOCMD) vet ./...

## tidy: 整理依赖
tidy:
	@echo "正在整理依赖..."
	$(GOMOD) tidy

## check-build: 格式化、检查、测试并构建
check-build: fmt vet test build

## clean: 清理构建产物
clean:
	@echo "正在清理..."
	$(GOCLEAN)
	@rm -rf $(BUILD_DIR)

## install: 安装二进制文件到 /usr/local/bin
install: build
	@echo "正在安装 $(BINARY_NAME)..."
	sudo cp $(BUILD_DIR)/$(BINARY_NAME) /usr/local/bin/
	sudo chmod +x /usr/local/bin/$(BINARY_NAME)

## uninstall: 从 /usr/local/bin 删除二进制文件
uninstall:
	@echo "正在卸载 $(BINARY_NAME)..."
	sudo rm -f /usr/local/bin/$(BINARY_NAME)

## init: 初始化配置
init: build
	@echo "正在初始化配置..."
	$(BUILD_DIR)/$(BINARY_NAME) init

## check: 检查配置
check: build
	@echo "正在检查配置..."
	$(BUILD_DIR)/$(BINARY_NAME) check

## help: 显示帮助
help:
	@echo "用法：make [target]"
	@echo ""
	@echo "目标："
	@sed -n 's/^## //p' $(MAKEFILE_LIST) | column -t -s ':' | sed 's/^/  /'
