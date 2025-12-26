# GB28181/ONVIF 信令服务器 Makefile
# 包含 ZLMediaKit 编译集成

.PHONY: all build build-zlm build-server build-frontend embed-frontend clean run test help

# 变量
GO := go

# 自动检测系统架构
UNAME_M := $(shell uname -m)
ifeq ($(UNAME_M),x86_64)
    DETECTED_ARCH := amd64
else ifeq ($(UNAME_M),amd64)
    DETECTED_ARCH := amd64
else ifeq ($(UNAME_M),aarch64)
    DETECTED_ARCH := arm64
else ifeq ($(UNAME_M),arm64)
    DETECTED_ARCH := arm64
else
    DETECTED_ARCH := $(shell go env GOARCH)
endif

# 平台设置 (可通过命令行覆盖: make build GOOS=windows GOARCH=amd64)
GOOS ?= $(shell go env GOOS)
GOARCH ?= $(DETECTED_ARCH)
BUILD_DIR := build
OUTPUT_DIR := dist
SERVER_NAME := gb28181-server
ZLM_EMBED_DIR := internal/zlm/embedded
FRONTEND_EMBED_DIR := internal/frontend

# 根据目标平台设置可执行文件后缀
ifeq ($(GOOS),windows)
    SERVER_EXT := .exe
else
    SERVER_EXT :=
endif

# 编译标志
LDFLAGS := -s -w
BUILD_TAGS := 

# 平台标识文件
ZLM_PLATFORM_FILE := $(ZLM_EMBED_DIR)/.platform
CURRENT_PLATFORM := $(GOOS)-$(GOARCH)

# 检查是否有嵌入式 ZLM
ifneq ($(wildcard $(ZLM_EMBED_DIR)/MediaServer),)
    BUILD_TAGS := embed_zlm
endif

# 检查是否有嵌入式前端
ifneq ($(wildcard $(FRONTEND_EMBED_DIR)/dist/index.html),)
    BUILD_TAGS := $(BUILD_TAGS) embed_frontend
endif

# 默认目标（包含 ZLM 和前端嵌入）
all: build-all

# 完整构建（包含 ZLM 和前端嵌入）
build-all: build-zlm embed-frontend build-server
	@echo "✓ 完整构建完成（ZLM + 前端已嵌入）"

# 只构建服务器（不含 ZLM 编译，但会嵌入前端）
build: embed-frontend build-server
	@echo "✓ 服务器构建完成（前端已嵌入）"

# 快速构建（不嵌入前端，用于开发）
build-quick: build-server
	@echo "✓ 快速构建完成（未嵌入前端）"

# 构建服务器
build-server: check-zlm-platform
	@echo ">>> 检测系统架构: $(UNAME_M) -> $(DETECTED_ARCH)"
	@echo ">>> 构建 Go 服务器 ($(GOOS)/$(GOARCH))..."
	@mkdir -p $(OUTPUT_DIR)
	CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) $(GO) build \
		-ldflags "$(LDFLAGS)" \
		-tags "$(BUILD_TAGS)" \
		-o $(OUTPUT_DIR)/$(SERVER_NAME)$(SERVER_EXT) \
		./cmd/server
	@echo "✓ 服务器构建完成: $(OUTPUT_DIR)/$(SERVER_NAME)$(SERVER_EXT)"

# 检查 ZLM 平台一致性（自动检测并按需重新编译）
check-zlm-platform:
	@if [ -f "$(ZLM_EMBED_DIR)/MediaServer" ]; then \
		ZLM_FILE_INFO=$$(file $(ZLM_EMBED_DIR)/MediaServer); \
		if echo "$$ZLM_FILE_INFO" | grep -q "x86-64\|x86_64"; then \
			ZLM_DETECTED_ARCH="amd64"; \
		elif echo "$$ZLM_FILE_INFO" | grep -q "aarch64\|ARM aarch64\|ARM64"; then \
			ZLM_DETECTED_ARCH="arm64"; \
		elif echo "$$ZLM_FILE_INFO" | grep -q "386\|i386\|i686"; then \
			ZLM_DETECTED_ARCH="386"; \
		else \
			ZLM_DETECTED_ARCH="unknown"; \
		fi; \
		echo ">>> ZLM 二进制检测: $$ZLM_DETECTED_ARCH (目标: $(GOARCH))"; \
		if [ "$$ZLM_DETECTED_ARCH" != "$(GOARCH)" ]; then \
			echo "⚠ ZLM 平台不匹配: $$ZLM_DETECTED_ARCH != $(GOARCH)"; \
			echo ">>> 自动重新编译 ZLM for $(CURRENT_PLATFORM)..."; \
			$(MAKE) build-zlm; \
		else \
			echo "✓ ZLM 平台匹配: $(CURRENT_PLATFORM)"; \
			echo "$(CURRENT_PLATFORM)" > $(ZLM_PLATFORM_FILE); \
		fi \
	else \
		echo "ℹ 未找到嵌入式 ZLM ($(ZLM_EMBED_DIR)/MediaServer)"; \
		echo ">>> 如需嵌入 ZLM，请先运行: make build-zlm"; \
	fi

# 构建 ZLMediaKit
build-zlm:
	@echo ">>> 编译 ZLMediaKit for $(CURRENT_PLATFORM)..."
	@chmod +x scripts/build_zlm.sh
	@./scripts/build_zlm.sh all
	@echo "$(CURRENT_PLATFORM)" > $(ZLM_PLATFORM_FILE)
	@echo "✓ ZLMediaKit 编译完成 ($(CURRENT_PLATFORM))"

# 只下载 ZLM 源码
download-zlm:
	@chmod +x scripts/build_zlm.sh
	@./scripts/build_zlm.sh download

# 构建前端（仅构建，不嵌入）
build-frontend:
	@echo ">>> 构建前端..."
	@cd frontend && npm install && npm run build
	@echo "✓ 前端构建完成"

# 嵌入前端到服务器
embed-frontend: build-frontend
	@echo ">>> 嵌入前端文件..."
	@rm -rf $(FRONTEND_EMBED_DIR)/dist
	@mkdir -p $(FRONTEND_EMBED_DIR)/dist
	@cp -r frontend/dist/* $(FRONTEND_EMBED_DIR)/dist/
	@echo "✓ 前端文件已嵌入到 $(FRONTEND_EMBED_DIR)/dist"

# 清理嵌入的前端文件
clean-frontend-embed:
	@echo ">>> 清理嵌入的前端文件..."
	@rm -rf $(FRONTEND_EMBED_DIR)/dist
	@echo "✓ 嵌入的前端文件已清理"

# 跨平台编译快捷方式
build-linux-amd64:
	@$(MAKE) build-server GOOS=linux GOARCH=amd64

build-linux-arm64:
	@$(MAKE) build-server GOOS=linux GOARCH=arm64

build-windows-amd64:
	@$(MAKE) build-server GOOS=windows GOARCH=amd64

build-darwin-amd64:
	@$(MAKE) build-server GOOS=darwin GOARCH=amd64

build-darwin-arm64:
	@$(MAKE) build-server GOOS=darwin GOARCH=arm64

# 编译所有平台
build-all-platforms:
	@echo ">>> 编译所有平台..."
	@$(MAKE) build-linux-amd64
	@$(MAKE) build-linux-arm64
	@$(MAKE) build-windows-amd64
	@$(MAKE) build-darwin-amd64
	@$(MAKE) build-darwin-arm64
	@echo "✓ 所有平台编译完成"

# 打包发布 (当前平台，前端已嵌入二进制)
package: embed-frontend build-server
	@echo ">>> 打包发布版本..."
	@mkdir -p $(OUTPUT_DIR)/release
	@RELEASE_NAME=$(SERVER_NAME)-$(CURRENT_PLATFORM); \
	RELEASE_DIR=$(OUTPUT_DIR)/release/$$RELEASE_NAME; \
	rm -rf $$RELEASE_DIR && mkdir -p $$RELEASE_DIR; \
	echo ">>> 复制服务器程序（含嵌入式前端）..."; \
	cp $(OUTPUT_DIR)/$(SERVER_NAME)$(SERVER_EXT) $$RELEASE_DIR/; \
	echo ">>> 复制配置文件..."; \
	mkdir -p $$RELEASE_DIR/configs; \
	cp configs/config.yaml $$RELEASE_DIR/configs/; \
	echo ">>> 复制启动脚本..."; \
	cp start.sh $$RELEASE_DIR/ 2>/dev/null || true; \
	cp quick_start.sh $$RELEASE_DIR/ 2>/dev/null || true; \
	echo ">>> 复制文档..."; \
	cp README.md $$RELEASE_DIR/ 2>/dev/null || true; \
	echo ">>> 创建目录结构..."; \
	mkdir -p $$RELEASE_DIR/logs $$RELEASE_DIR/recordings; \
	echo ">>> 打包压缩..."; \
	cd $(OUTPUT_DIR)/release && tar -czvf $$RELEASE_NAME.tar.gz $$RELEASE_NAME; \
	echo "✓ 打包完成: $(OUTPUT_DIR)/release/$$RELEASE_NAME.tar.gz"

# 打包发布 (包含嵌入式 ZLM，前端已嵌入二进制)
package-with-zlm: build-all
	@echo ">>> 打包发布版本 (含 ZLM)..."
	@mkdir -p $(OUTPUT_DIR)/release
	@RELEASE_NAME=$(SERVER_NAME)-$(CURRENT_PLATFORM)-with-zlm; \
	RELEASE_DIR=$(OUTPUT_DIR)/release/$$RELEASE_NAME; \
	rm -rf $$RELEASE_DIR && mkdir -p $$RELEASE_DIR; \
	echo ">>> 复制服务器程序（含嵌入式前端和ZLM）..."; \
	cp $(OUTPUT_DIR)/$(SERVER_NAME)$(SERVER_EXT) $$RELEASE_DIR/; \
	echo ">>> 复制配置文件..."; \
	mkdir -p $$RELEASE_DIR/configs; \
	cp configs/config.yaml $$RELEASE_DIR/configs/; \
	echo ">>> 复制启动脚本..."; \
	cp start.sh $$RELEASE_DIR/ 2>/dev/null || true; \
	cp quick_start.sh $$RELEASE_DIR/ 2>/dev/null || true; \
	echo ">>> 复制文档..."; \
	cp README.md $$RELEASE_DIR/ 2>/dev/null || true; \
	echo ">>> 创建目录结构..."; \
	mkdir -p $$RELEASE_DIR/logs $$RELEASE_DIR/recordings; \
	echo ">>> 打包压缩..."; \
	cd $(OUTPUT_DIR)/release && tar -czvf $$RELEASE_NAME.tar.gz $$RELEASE_NAME; \
	echo "✓ 打包完成: $(OUTPUT_DIR)/release/$$RELEASE_NAME.tar.gz"

# 开发模式运行
run: build-server
	@echo ">>> 启动服务器..."
	@$(OUTPUT_DIR)/$(SERVER_NAME)$(SERVER_EXT) -config configs/config.yaml

# 开发模式（不启动 ZLM）
run-no-zlm: build-server
	@echo ">>> 启动服务器 (无 ZLM)..."
	@$(OUTPUT_DIR)/$(SERVER_NAME)$(SERVER_EXT) -config configs/config.yaml --no-zlm

# 测试
test:
	@echo ">>> 运行测试..."
	@$(GO) test -v ./...

# 检查代码
lint:
	@echo ">>> 代码检查..."
	@$(GO) vet ./...
	@command -v golint >/dev/null 2>&1 && golint ./... || echo "golint 未安装，跳过"

# 清理
clean:
	@echo ">>> 清理构建文件..."
	@rm -rf $(BUILD_DIR) $(OUTPUT_DIR)
	@rm -f server server.pid
	@echo "✓ 清理完成"

# 深度清理（包含 ZLM 源码和嵌入的前端）
clean-all: clean clean-frontend-embed
	@echo ">>> 清理 ZLM 编译文件..."
	@chmod +x scripts/build_zlm.sh 2>/dev/null || true
	@./scripts/build_zlm.sh clean 2>/dev/null || true
	@rm -rf $(ZLM_EMBED_DIR)/MediaServer $(ZLM_EMBED_DIR)/www $(ZLM_EMBED_DIR)/*.template $(ZLM_PLATFORM_FILE)
	@echo "✓ 深度清理完成"

# 安装依赖
deps:
	@echo ">>> 安装 Go 依赖..."
	@$(GO) mod download
	@$(GO) mod tidy
	@echo "✓ 依赖安装完成"

# 安装 ZLM 编译依赖
deps-zlm:
	@echo ">>> 安装 ZLM 编译依赖..."
	@chmod +x scripts/build_zlm.sh
	@./scripts/build_zlm.sh deps

# 版本信息
version:
	@echo "Go 版本: $(shell $(GO) version)"
	@echo "构建目标: $(GOOS)/$(GOARCH)"
	@if [ -f "$(ZLM_EMBED_DIR)/VERSION" ]; then \
		echo "ZLM 版本: $$(cat $(ZLM_EMBED_DIR)/VERSION)"; \
	else \
		echo "ZLM 版本: 未编译"; \
	fi
	@if [ -f "$(ZLM_PLATFORM_FILE)" ]; then \
		echo "ZLM 平台: $$(cat $(ZLM_PLATFORM_FILE))"; \
	fi
	@if [ -d "$(FRONTEND_EMBED_DIR)/dist" ]; then \
		echo "前端嵌入: 已嵌入"; \
	else \
		echo "前端嵌入: 未嵌入（将使用本地文件）"; \
	fi

# 帮助
help:
	@echo "GB28181/ONVIF 信令服务器 构建系统"
	@echo ""
	@echo "用法: make [目标]"
	@echo ""
	@echo "主要目标:"
	@echo "  all             完整构建（包含 ZLM + 嵌入前端）"
	@echo "  build           构建服务器（嵌入前端）"
	@echo "  build-quick     快速构建（不嵌入前端，用于开发）"
	@echo "  build-all       完整构建（包含 ZLM + 嵌入前端）"
	@echo "  build-server    只构建 Go 服务器（不嵌入）"
	@echo "  build-zlm       编译 ZLMediaKit"
	@echo "  build-frontend  构建前端（不嵌入）"
	@echo "  embed-frontend  构建并嵌入前端"
	@echo ""
	@echo "跨平台编译:"
	@echo "  make build GOOS=linux GOARCH=amd64    # Linux 64位"
	@echo "  make build GOOS=linux GOARCH=arm64    # Linux ARM64"
	@echo "  make build GOOS=windows GOARCH=amd64  # Windows 64位"
	@echo "  make build GOOS=darwin GOARCH=amd64   # macOS Intel"
	@echo "  make build GOOS=darwin GOARCH=arm64   # macOS Apple Silicon"
	@echo ""
	@echo "运行:"
	@echo "  run           构建并运行服务器"
	@echo "  run-no-zlm    运行服务器 (不启动 ZLM)"
	@echo ""
	@echo "开发:"
	@echo "  test          运行测试"
	@echo "  lint          代码检查"
	@echo "  deps          安装 Go 依赖"
	@echo "  deps-zlm      安装 ZLM 编译依赖"
	@echo ""
	@echo "清理:"
	@echo "  clean         清理构建文件"
	@echo "  clean-all     深度清理 (包含 ZLM)"
	@echo ""
	@echo "其他:"
	@echo "  version       显示版本信息"
	@echo "  help          显示此帮助"
