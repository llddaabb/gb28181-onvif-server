# GB28181/ONVIF 信令服务器 Makefile
# 包含 ZLMediaKit 编译集成

.PHONY: all build build-zlm build-server build-frontend clean run test help

# 变量
GO := go
GOOS := linux
GOARCH := amd64
BUILD_DIR := build
OUTPUT_DIR := dist
SERVER_NAME := gb28181-server
ZLM_EMBED_DIR := internal/zlm/embedded

# 编译标志
LDFLAGS := -s -w
BUILD_TAGS := 

# 检查是否有嵌入式 ZLM
ifneq ($(wildcard $(ZLM_EMBED_DIR)/MediaServer),)
    BUILD_TAGS := embed_zlm
endif

# 默认目标
all: build

# 完整构建（包含 ZLM）
build-all: build-zlm build-server build-frontend
	@echo "✓ 完整构建完成"

# 只构建服务器（不含 ZLM 编译）
build: build-server
	@echo "✓ 服务器构建完成"

# 构建服务器
build-server:
	@echo ">>> 构建 Go 服务器..."
	@mkdir -p $(OUTPUT_DIR)
	CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) $(GO) build \
		-ldflags "$(LDFLAGS)" \
		-tags "$(BUILD_TAGS)" \
		-o $(OUTPUT_DIR)/$(SERVER_NAME) \
		./cmd/server
	@echo "✓ 服务器构建完成: $(OUTPUT_DIR)/$(SERVER_NAME)"

# 构建 ZLMediaKit
build-zlm:
	@echo ">>> 编译 ZLMediaKit..."
	@chmod +x scripts/build_zlm.sh
	@./scripts/build_zlm.sh all
	@echo "✓ ZLMediaKit 编译完成"

# 只下载 ZLM 源码
download-zlm:
	@chmod +x scripts/build_zlm.sh
	@./scripts/build_zlm.sh download

# 构建前端
build-frontend:
	@echo ">>> 构建前端..."
	@cd frontend && npm install && npm run build
	@echo "✓ 前端构建完成"

# 开发模式运行
run: build-server
	@echo ">>> 启动服务器..."
	@$(OUTPUT_DIR)/$(SERVER_NAME) -config configs/config.yaml

# 开发模式（不启动 ZLM）
run-no-zlm: build-server
	@echo ">>> 启动服务器 (无 ZLM)..."
	@$(OUTPUT_DIR)/$(SERVER_NAME) -config configs/config.yaml --no-zlm

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

# 深度清理（包含 ZLM 源码）
clean-all: clean
	@echo ">>> 清理 ZLM 编译文件..."
	@chmod +x scripts/build_zlm.sh 2>/dev/null || true
	@./scripts/build_zlm.sh clean 2>/dev/null || true
	@rm -rf $(ZLM_EMBED_DIR)/MediaServer $(ZLM_EMBED_DIR)/www $(ZLM_EMBED_DIR)/*.template
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

# 帮助
help:
	@echo "GB28181/ONVIF 信令服务器 构建系统"
	@echo ""
	@echo "用法: make [目标]"
	@echo ""
	@echo "主要目标:"
	@echo "  all           构建服务器 (默认)"
	@echo "  build-all     完整构建 (包含 ZLM)"
	@echo "  build-server  只构建 Go 服务器"
	@echo "  build-zlm     编译 ZLMediaKit"
	@echo "  build-frontend 构建前端"
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
