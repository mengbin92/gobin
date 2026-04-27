.PHONY: help build build-all test test-coverage clean install release-local

# 版本信息
VERSION ?= $(shell git describe --tags --exact-match 2>/dev/null || echo "dev")
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE ?= $(shell date -u '+%Y-%m-%d_%H:%M:%S')

# Go 参数
GOCMD = go
GOBUILD = $(GOCMD) build
GOTEST = $(GOCMD) test
GOCLEAN = $(GOCMD) clean
GOINSTALL = $(GOCMD) install
LDFLAGS = -ldflags "-s -w -X 'github.com/mengbin92/gobin/cmd/gobin/commands.Version=${VERSION}' -X 'github.com/mengbin92/gobin/cmd/gobin/commands.Commit=${COMMIT}' -X 'github.com/mengbin92/gobin/cmd/gobin/commands.BuildDate=${BUILD_DATE}'"

# 帮助信息
help:
	@echo "Gobin Makefile"
	@echo "=============="
	@echo ""
	@echo "可用目标:"
	@echo "  build         - 构建当前平台的二进制文件"
	@echo "  build-all     - 构建所有支持平台的二进制文件"
	@echo "  test          - 运行所有测试"
	@echo "  test-coverage - 运行测试并生成覆盖率报告"
	@echo "  clean         - 清理构建产物"
	@echo "  install       - 安装到 $GOPATH/bin"
	@echo "  release-local - 本地构建所有平台并打包"
	@echo "  help          - 显示帮助信息"
	@echo ""
	@echo "示例:"
	@echo "  make build VERSION=v1.0.0"
	@echo "  make test"
	@echo "  make build-all"

# 构建当前平台
build:
	@echo "🔨 Building Gobin $(VERSION)..."
	$(GOBUILD) $(LDFLAGS) -o gobin ./cmd/gobin
	@echo "✅ Build complete: ./gobin"
	@ls -lh gobin

# 构建所有平台
build-all:
	@echo "📦 Building for all platforms..."
	@./scripts/build-all.sh

# 运行测试
test:
	@echo "🧪 Running tests..."
	$(GOTEST) -v ./internal/parser/... ./internal/config/... ./internal/generator/...

# 运行测试并显示覆盖率
test-coverage:
	@echo "🧪 Running tests with coverage..."
	$(GOTEST) -v -cover ./internal/parser/... ./internal/config/... ./internal/generator/...
	@echo ""
	@echo "📊 Coverage summary:"
	$(GOTEST) -cover ./internal/parser/... ./internal/config/... ./internal/generator/... | grep coverage

# 清理
clean:
	@echo "🧹 Cleaning..."
	$(GOCLEAN)
	@rm -rf dist/
	@rm -f gobin
	@echo "✅ Clean complete"

# 安装到 GOPATH/bin
install:
	@echo "📦 Installing Gobin..."
	$(GOINSTALL) ./cmd/gobin
	@echo "✅ Installed to: $(shell go env GOPATH)/bin/gobin"

# 本地发布（构建所有平台并创建压缩包）
release-local: build-all
	@echo "📦 Creating release archives..."
	@cd dist && \
		for file in gobin-v*; do \
			if [[ "$$file" == *".exe" ]]; then \
				zip "$${file%.exe}.zip" "$$file"; \
				echo "  📁 Created $${file%.exe}.zip"; \
			else \
				tar czf "$$file.tar.gz" "$$file"; \
				echo "  📁 Created $$file.tar.gz"; \
			fi; \
		done
	@echo ""
	@echo "✅ Release archives created in dist/"
	@ls -lh dist/*.{zip,tar.gz}

# 快速测试（运行示例站点）
example:
	@echo "🚀 Running example site..."
	@cd example-site && gobin build && gobin serve
