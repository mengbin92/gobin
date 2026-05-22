.PHONY: help build build-all test lint test-coverage benchmark benchmark-check clean install release-local

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
BENCH_TIME ?= 100ms
BENCH_OUTPUT ?= benchmark-results.txt
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
	@echo "  lint          - 运行格式和静态检查"
	@echo "  test-coverage - 运行测试并生成覆盖率报告"
	@echo "  benchmark     - 运行性能基准并写入 benchmark-results.txt"
	@echo "  benchmark-check - 校验 benchmark-results.txt 的性能阈值"
	@echo "  clean         - 清理构建产物"
	@echo "  install       - 安装到 $$GOPATH/bin"
	@echo "  release-local - 本地构建所有平台并打包"
	@echo "  help          - 显示帮助信息"
	@echo ""
	@echo "示例:"
	@echo "  make build VERSION=v1.2.0"
	@echo "  make test"
	@echo "  make benchmark"
	@echo "  make build-all"

# 构建当前平台
build:
	@echo "[build] Building Gobin $(VERSION)..."
	$(GOBUILD) $(LDFLAGS) -o gobin ./cmd/gobin
	@echo "[ok] Build complete: ./gobin"
	@ls -lh gobin

# 构建所有平台
build-all:
	@echo "[package] Building for all platforms..."
	@./scripts/build-all.sh

# 运行测试
test:
	@echo "[test] Running tests..."
	$(GOTEST) -v ./...

lint:
	@echo "[lint] Checking gofmt..."
	@test -z "$$(gofmt -l $$(find . -name '*.go' -not -path './dist/*'))"
	@echo "[lint] Running go vet..."
	go vet ./...

# 运行测试并显示覆盖率
test-coverage:
	@echo "[test] Running tests with coverage..."
	$(GOTEST) -v -cover ./internal/parser/... ./internal/config/... ./internal/generator/...
	@echo ""
	@echo "[coverage] Coverage summary:"
	$(GOTEST) -cover ./internal/parser/... ./internal/config/... ./internal/generator/... | grep coverage

# 运行性能基准并保存结果
benchmark:
	@echo "[bench] Running benchmark baseline..."
	$(GOTEST) -run '^$$' -bench=. -benchmem -benchtime=$(BENCH_TIME) -count=1 ./... > $(BENCH_OUTPUT)
	@cat $(BENCH_OUTPUT)
	@echo "[file] Benchmark results written to $(BENCH_OUTPUT)"

benchmark-check:
	@echo "[bench] Checking benchmark thresholds..."
	@./scripts/check-benchmark.sh $(BENCH_OUTPUT)

# 清理
clean:
	@echo "[clean] Cleaning..."
	$(GOCLEAN)
	@rm -rf dist/
	@rm -f gobin
	@rm -f benchmark-results.txt
	@echo "[ok] Clean complete"

# 安装到 GOPATH/bin
install:
	@echo "[package] Installing Gobin..."
	$(GOINSTALL) ./cmd/gobin
	@echo "[ok] Installed to: $(shell go env GOPATH)/bin/gobin"

# 本地发布（构建所有平台、打包并生成校验和）
release-local: build-all
	@echo "[ok] Release archives and checksums created in dist/"
	@ls -lh dist/*.zip dist/*.tar.gz dist/SHA256SUMS

# 快速测试（运行示例站点）
example:
	@echo "[run] Running example site..."
	@cd example-site && gobin build && gobin serve
