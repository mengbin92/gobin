# Gobin 统一日志系统 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 为 Gobin 引入基于 `log/slog` 的统一结构化日志系统，分离用户输出与诊断日志，并为 internal 层提供可观测性。

**Architecture:** 新增零依赖的 `internal/log` 包封装 slog，提供全局 logger；CLI 通过 persistent flags（`--verbose`/`--log-format`/`--log-file`）在 `PersistentPreRun` 初始化全局 logger；commands 层把诊断信息从 stdout 迁到 slog（stderr/文件），internal 层用 `GetDefault().With("component", ...)` 埋点而不改函数签名。

**Tech Stack:** Go 1.25 标准库 `log/slog`、`github.com/spf13/cobra`。无新增外部依赖。

**项目根目录:** `~/vscode/mengbin/gobin`（module `github.com/mengbin92/gobin`）。下文路径均相对该根目录。

**关键约束:** `internal/log` 只依赖标准库，**不得 import 任何其他 internal 包**（否则与即将 import 它的 config/parser/generator 形成 import cycle）。

---

## File Structure

| 文件 | 职责 | 动作 |
|------|------|------|
| `internal/log/writer.go` | `resolveWriter`：把 output 字符串解析为 io.Writer | Create |
| `internal/log/log.go` | Options/Level/Format 类型、`New`/`NewFromCLI`/`WithComponent`、全局 logger 与便捷函数 | Create |
| `internal/log/context.go` | `FromContext`/`IntoContext` | Create |
| `internal/log/log_test.go` | `internal/log` 单元测试 | Create |
| `cmd/gobin/commands/logging.go` | persistent flag 变量 + `AddGlobalFlags` + `InitLogging`（verbose 统一落点） | Create |
| `cmd/gobin/main.go` | 注册全局 flag + PersistentPreRun 初始化 logger | Modify |
| `cmd/gobin/commands/build.go` | 分离用户输出 / 诊断日志 | Modify |
| `cmd/gobin/commands/serve.go` | 删除局部 verbose flag，改用全局 `verbose`；构建诊断日志 | Modify |
| `cmd/gobin/commands/serve_server.go` | 替换 `log.Printf` 为 slog | Modify |
| `cmd/gobin/commands/serve_watcher.go` | `serveVerbose`→`verbose`；warning 改 slog | Modify |
| `cmd/gobin/commands/check.go` | 校验过程诊断日志 | Modify |
| `cmd/gobin/commands/init.go` `new.go` | 脚手架关键步骤日志 | Modify |
| `internal/generator/generator.go` | 生成入口埋点（component=generator） | Modify |
| `internal/parser/parser.go` | 解析入口埋点（component=parser） | Modify |
| `internal/config/config.go` | 加载/校验埋点（component=config） | Modify |

---

## Task 1: log 包 — writer.go (resolveWriter)

**Files:**
- Create: `internal/log/writer.go`
- Test: `internal/log/log_test.go`

- [ ] **Step 1: 写失败测试**

创建 `internal/log/log_test.go`：

```go
package log

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveWriter(t *testing.T) {
	if got := resolveWriter("stdout"); got != os.Stdout {
		t.Errorf("resolveWriter(stdout) = %v, want os.Stdout", got)
	}
	if got := resolveWriter("stderr"); got != os.Stderr {
		t.Errorf("resolveWriter(stderr) = %v, want os.Stderr", got)
	}
	if got := resolveWriter(""); got != os.Stderr {
		t.Errorf("resolveWriter(\"\") = %v, want os.Stderr", got)
	}
}

func TestResolveWriterFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.log")
	w := resolveWriter(path)
	f, ok := w.(*os.File)
	if !ok {
		t.Fatalf("resolveWriter(path) returned %T, want *os.File", w)
	}
	defer f.Close()
	if _, err := f.WriteString("hello\n"); err != nil {
		t.Fatalf("write to log file failed: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read log file failed: %v", err)
	}
	if string(data) != "hello\n" {
		t.Errorf("log file content = %q, want %q", string(data), "hello\n")
	}
}

func TestResolveWriterBadPathFallsBackToStderr(t *testing.T) {
	// A path inside a non-existent directory cannot be opened.
	bad := filepath.Join(t.TempDir(), "nope", "deeper", "x.log")
	if got := resolveWriter(bad); got != os.Stderr {
		t.Errorf("resolveWriter(bad) = %v, want os.Stderr fallback", got)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd ~/vscode/mengbin/gobin && go test ./internal/log/ -run TestResolveWriter -v`
Expected: 编译失败 —— `undefined: resolveWriter`。

- [ ] **Step 3: 实现 writer.go**

创建 `internal/log/writer.go`：

```go
package log

import (
	"io"
	"os"
)

// resolveWriter maps an Options.Output string to an io.Writer.
// "stdout" -> os.Stdout, "stderr"/"" -> os.Stderr, otherwise a file
// opened in append mode. On open failure it warns and falls back to stderr.
func resolveWriter(output string) io.Writer {
	switch output {
	case "stdout":
		return os.Stdout
	case "stderr", "":
		return os.Stderr
	default:
		f, err := os.OpenFile(output, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			os.Stderr.WriteString("warning: cannot open log file " + output + ", falling back to stderr\n")
			return os.Stderr
		}
		return f
	}
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `cd ~/vscode/mengbin/gobin && go test ./internal/log/ -run TestResolveWriter -v`
Expected: PASS（3 个子测试）。

- [ ] **Step 5: 提交**

```bash
cd ~/vscode/mengbin/gobin
git add internal/log/writer.go internal/log/log_test.go
git commit -m "feat(log): add resolveWriter for stderr/stdout/file output"
```

---

## Task 2: log 包 — log.go 核心 (New / Level / Format)

**Files:**
- Create: `internal/log/log.go`
- Test: `internal/log/log_test.go`（追加）

- [ ] **Step 1: 写失败测试**

在 `internal/log/log_test.go` 顶部 import 块改为：

```go
import (
	"bytes"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)
```

在文件末尾追加：

```go
// newTestLogger builds a logger writing into buf with the given format/level.
func newTestLogger(buf *bytes.Buffer, format Format, level slog.Level) *slog.Logger {
	var h slog.Handler
	opts := &slog.HandlerOptions{Level: level}
	if format == FormatJSON {
		h = slog.NewJSONHandler(buf, opts)
	} else {
		h = slog.NewTextHandler(buf, opts)
	}
	return slog.New(h)
}

func TestParseLevel(t *testing.T) {
	cases := map[Level]slog.Level{
		LevelDebug: slog.LevelDebug,
		LevelInfo:  slog.LevelInfo,
		LevelWarn:  slog.LevelWarn,
		LevelError: slog.LevelError,
		Level("bogus"): slog.LevelInfo, // unknown -> info
	}
	for in, want := range cases {
		if got := parseLevel(in); got != want {
			t.Errorf("parseLevel(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestNewTextHandlerFiltersByLevel(t *testing.T) {
	path := filepath.Join(t.TempDir(), "out.log")
	logger := New(Options{Level: LevelInfo, Format: FormatText, Output: path})
	logger.Debug("debug-msg")
	logger.Info("info-msg")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	out := string(data)
	if strings.Contains(out, "debug-msg") {
		t.Errorf("INFO logger should not emit debug, got: %s", out)
	}
	if !strings.Contains(out, "info-msg") {
		t.Errorf("INFO logger should emit info, got: %s", out)
	}
}

func TestNewJSONFormat(t *testing.T) {
	path := filepath.Join(t.TempDir(), "out.json")
	logger := New(Options{Level: LevelInfo, Format: FormatJSON, Output: path})
	logger.Info("hello", "k", "v")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var rec map[string]any
	if err := json.Unmarshal(data[:len(data)-1], &rec); err != nil {
		t.Fatalf("output is not valid JSON: %v (%q)", err, string(data))
	}
	if rec["msg"] != "hello" || rec["k"] != "v" {
		t.Errorf("unexpected JSON record: %v", rec)
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd ~/vscode/mengbin/gobin && go test ./internal/log/ -run 'TestParseLevel|TestNew' -v`
Expected: 编译失败 —— `undefined: parseLevel`、`undefined: New`、`undefined: Options` 等。

- [ ] **Step 3: 实现 log.go（本任务先实现类型 + parseLevel + New）**

创建 `internal/log/log.go`：

```go
package log

import (
	"log/slog"
)

// Level is a string log level that maps onto slog.Level.
type Level string

const (
	LevelDebug Level = "debug"
	LevelInfo  Level = "info"
	LevelWarn  Level = "warn"
	LevelError Level = "error"
)

// Format selects the slog handler used for output.
type Format string

const (
	FormatText Format = "text" // terminal-friendly (default)
	FormatJSON Format = "json" // machine-readable (CI / pipelines)
)

// Options configures a logger built by New.
type Options struct {
	Level     Level  // default info
	Format    Format // default text
	Output    string // "stderr" (default), "stdout", or a file path
	AddSource bool   // include source location (only meaningful at debug)
}

// Default returns the baseline configuration: INFO, text, stderr.
func Default() Options {
	return Options{Level: LevelInfo, Format: FormatText, Output: "stderr"}
}

func parseLevel(l Level) slog.Level {
	switch l {
	case LevelDebug:
		return slog.LevelDebug
	case LevelWarn:
		return slog.LevelWarn
	case LevelError:
		return slog.LevelError
	case LevelInfo:
		return slog.LevelInfo
	default:
		return slog.LevelInfo
	}
}

// New builds a *slog.Logger from opts.
func New(opts Options) *slog.Logger {
	level := parseLevel(opts.Level)
	handlerOpts := &slog.HandlerOptions{
		Level:     level,
		AddSource: opts.AddSource && level <= slog.LevelDebug,
	}
	writer := resolveWriter(opts.Output)

	var handler slog.Handler
	switch opts.Format {
	case FormatJSON:
		handler = slog.NewJSONHandler(writer, handlerOpts)
	default:
		handler = slog.NewTextHandler(writer, handlerOpts)
	}
	return slog.New(handler)
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `cd ~/vscode/mengbin/gobin && go test ./internal/log/ -run 'TestParseLevel|TestNew' -v`
Expected: PASS。

- [ ] **Step 5: 提交**

```bash
cd ~/vscode/mengbin/gobin
git add internal/log/log.go internal/log/log_test.go
git commit -m "feat(log): add Options/Level/Format types and New constructor"
```

---

## Task 3: log 包 — NewFromCLI / WithComponent / 全局 logger / 便捷函数

**Files:**
- Modify: `internal/log/log.go`
- Test: `internal/log/log_test.go`（追加）

- [ ] **Step 1: 写失败测试**

在 `internal/log/log_test.go` 末尾追加：

```go
func TestNewFromCLIVerboseEnablesDebug(t *testing.T) {
	path := filepath.Join(t.TempDir(), "v.log")
	logger := NewFromCLI(true, "text", path)
	logger.Debug("dbg-line")
	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), "dbg-line") {
		t.Errorf("verbose logger should emit debug, got: %s", string(data))
	}
}

func TestNewFromCLIDefaultsHideDebug(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nv.log")
	logger := NewFromCLI(false, "", path)
	logger.Debug("dbg-line")
	logger.Info("info-line")
	data, _ := os.ReadFile(path)
	out := string(data)
	if strings.Contains(out, "dbg-line") {
		t.Errorf("non-verbose logger should hide debug, got: %s", out)
	}
	if !strings.Contains(out, "info-line") {
		t.Errorf("non-verbose logger should show info, got: %s", out)
	}
}

func TestWithComponentAddsField(t *testing.T) {
	var buf bytes.Buffer
	base := newTestLogger(&buf, FormatJSON, slog.LevelInfo)
	logger := WithComponent(base, "parser")
	logger.Info("scanned")
	var rec map[string]any
	json.Unmarshal(buf.Bytes()[:buf.Len()-1], &rec)
	if rec["component"] != "parser" {
		t.Errorf("component field = %v, want parser", rec["component"])
	}
}

func TestSetAndGetDefault(t *testing.T) {
	orig := GetDefault()
	defer SetDefault(orig)

	var buf bytes.Buffer
	custom := newTestLogger(&buf, FormatText, slog.LevelInfo)
	SetDefault(custom)
	if GetDefault() != custom {
		t.Error("GetDefault did not return the logger set by SetDefault")
	}
	Info("via-convenience")
	if !strings.Contains(buf.String(), "via-convenience") {
		t.Errorf("convenience Info did not route to default logger, got: %s", buf.String())
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd ~/vscode/mengbin/gobin && go test ./internal/log/ -run 'NewFromCLI|WithComponent|SetAndGetDefault' -v`
Expected: 编译失败 —— `undefined: NewFromCLI` 等。

- [ ] **Step 3: 实现（追加到 log.go 末尾）**

在 `internal/log/log.go` 末尾追加：

```go
// NewFromCLI builds a logger from CLI flag values.
// verbose=true raises the level to debug and enables source locations.
func NewFromCLI(verbose bool, format, logFile string) *slog.Logger {
	opts := Default()
	if verbose {
		opts.Level = LevelDebug
		opts.AddSource = true
	}
	if format != "" {
		opts.Format = Format(format)
	}
	if logFile != "" {
		opts.Output = logFile
	}
	return New(opts)
}

// WithComponent returns a child logger tagged with the given component name.
func WithComponent(logger *slog.Logger, component string) *slog.Logger {
	return logger.With("component", component)
}

// ----- global default logger -----

var defaultLogger *slog.Logger

func init() {
	defaultLogger = New(Default())
}

// SetDefault installs l as the package default and the slog default.
func SetDefault(l *slog.Logger) {
	defaultLogger = l
	slog.SetDefault(l)
}

// GetDefault returns the current package default logger.
func GetDefault() *slog.Logger { return defaultLogger }

// Convenience helpers forwarding to the default logger.
func Debug(msg string, args ...any) { defaultLogger.Debug(msg, args...) }
func Info(msg string, args ...any)  { defaultLogger.Info(msg, args...) }
func Warn(msg string, args ...any)  { defaultLogger.Warn(msg, args...) }
func Error(msg string, args ...any) { defaultLogger.Error(msg, args...) }
```

- [ ] **Step 4: 运行测试确认通过**

Run: `cd ~/vscode/mengbin/gobin && go test ./internal/log/ -v`
Expected: 全部 PASS。

- [ ] **Step 5: 提交**

```bash
cd ~/vscode/mengbin/gobin
git add internal/log/log.go internal/log/log_test.go
git commit -m "feat(log): add NewFromCLI, WithComponent, and global default logger"
```

---

## Task 4: log 包 — context.go

**Files:**
- Create: `internal/log/context.go`
- Test: `internal/log/log_test.go`（追加）

- [ ] **Step 1: 写失败测试**

在 `internal/log/log_test.go` import 块追加 `"context"`，并在末尾追加：

```go
func TestContextRoundTrip(t *testing.T) {
	var buf bytes.Buffer
	logger := newTestLogger(&buf, FormatText, slog.LevelInfo)
	ctx := IntoContext(context.Background(), logger)
	if FromContext(ctx) != logger {
		t.Error("FromContext did not return the logger put via IntoContext")
	}
}

func TestFromContextFallsBackToDefault(t *testing.T) {
	if FromContext(context.Background()) != GetDefault() {
		t.Error("FromContext on empty context should return the default logger")
	}
}
```

- [ ] **Step 2: 运行测试确认失败**

Run: `cd ~/vscode/mengbin/gobin && go test ./internal/log/ -run 'Context' -v`
Expected: 编译失败 —— `undefined: IntoContext` / `FromContext`。

- [ ] **Step 3: 实现 context.go**

创建 `internal/log/context.go`：

```go
package log

import (
	"context"
	"log/slog"
)

type contextKey struct{}

// FromContext extracts a logger from ctx, or returns the default logger.
func FromContext(ctx context.Context) *slog.Logger {
	if l, ok := ctx.Value(contextKey{}).(*slog.Logger); ok {
		return l
	}
	return defaultLogger
}

// IntoContext returns a copy of ctx carrying the given logger.
func IntoContext(ctx context.Context, l *slog.Logger) context.Context {
	return context.WithValue(ctx, contextKey{}, l)
}
```

- [ ] **Step 4: 运行测试确认通过**

Run: `cd ~/vscode/mengbin/gobin && go test ./internal/log/ -v`
Expected: 全部 PASS。

- [ ] **Step 5: 提交**

```bash
cd ~/vscode/mengbin/gobin
git add internal/log/context.go internal/log/log_test.go
git commit -m "feat(log): add context propagation helpers"
```

---

## Task 5: CLI 集成 + verbose 统一

新增 `commands/logging.go` 持有全局 flag 变量（放 commands 包，便于 `serve_watcher` 复用 `verbose`），`main.go` 注册并在 PreRun 初始化。删除 serve 局部 verbose flag。

**Files:**
- Create: `cmd/gobin/commands/logging.go`
- Modify: `cmd/gobin/main.go`
- Modify: `cmd/gobin/commands/serve.go:67`（删除 verbose flag 注册）
- Modify: `cmd/gobin/commands/serve.go:117` 和 `serve_watcher.go:91`（`serveVerbose`→`verbose`）
- Modify: `cmd/gobin/commands/serve.go:45`（删除 `serveVerbose` 声明）

- [ ] **Step 1: 创建 logging.go**

创建 `cmd/gobin/commands/logging.go`：

```go
package commands

import (
	"github.com/mengbin92/gobin/internal/log"
	"github.com/spf13/cobra"
)

// Global logging flags, shared across all commands.
var (
	verbose   bool
	logFormat string
	logFile   string
)

// AddGlobalFlags registers the persistent logging flags on the root command.
func AddGlobalFlags(rootCmd *cobra.Command) {
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable debug logging")
	rootCmd.PersistentFlags().StringVar(&logFormat, "log-format", "text", "Log format: text or json")
	rootCmd.PersistentFlags().StringVar(&logFile, "log-file", "", "Write logs to file (default: stderr)")
}

// InitLogging builds the global logger from the parsed flag values.
func InitLogging() {
	log.SetDefault(log.NewFromCLI(verbose, logFormat, logFile))
}
```

- [ ] **Step 2: 修改 main.go**

将 `cmd/gobin/main.go` 的 `init()` 改为（在现有 AddCommand 之后追加注册与 PreRun）：

```go
func init() {
	rootCmd.AddCommand(commands.BuildCmd)
	rootCmd.AddCommand(commands.VersionCmd)
	rootCmd.AddCommand(commands.InitCmd)
	rootCmd.AddCommand(commands.ServeCmd)
	rootCmd.AddCommand(commands.NewCmd)
	rootCmd.AddCommand(commands.CheckCmd)

	commands.AddGlobalFlags(rootCmd)
	rootCmd.PersistentPreRun = func(cmd *cobra.Command, args []string) {
		commands.InitLogging()
	}
}
```

- [ ] **Step 3: 删除 serve 的局部 verbose flag 与变量**

在 `cmd/gobin/commands/serve.go`：

1. 删除第 45 行的 `serveVerbose bool`（保留同 `var (...)` 块内其它项）。删除后该块为：

```go
var (
	servePort  int
	serveWatch bool
	serveDrafts bool
	serveClean bool
	serveReload bool
)
```

2. 删除 `init()` 中这一行（原 serve.go:67）：

```go
	ServeCmd.Flags().BoolVarP(&serveVerbose, "verbose", "v", false, "Verbose output")
```

3. 把 `runServeWithOps` 中（原 serve.go:117）：

```go
		runtime.verbose = serveVerbose
```

改为：

```go
		runtime.verbose = verbose
```

- [ ] **Step 4: 修改 serve_watcher.go 的 serveVerbose 引用**

在 `cmd/gobin/commands/serve_watcher.go:91`，把：

```go
		verbose:     serveVerbose,
```

改为：

```go
		verbose:     verbose,
```

- [ ] **Step 5: 运行构建与测试确认无残留引用**

Run: `cd ~/vscode/mengbin/gobin && grep -rn serveVerbose cmd/ ; go build ./... && go vet ./...`
Expected: `grep` 在非测试文件中无输出（若 `serve_test.go` 仍引用 `serveVerbose`，进入 Step 6 修复）；`go build`/`go vet` 通过。

- [ ] **Step 6: 修复测试中对 serveVerbose 的引用（如有）**

`serve_test.go` 中使用的是 `serveRuntime{verbose: true}` 结构体字段（非 `serveVerbose` 包变量），不受影响。若 Step 5 的 grep 显示 `serve_test.go` 引用了 `serveVerbose` 包变量，将其替换为 `verbose`。否则跳过本步。

Run: `cd ~/vscode/mengbin/gobin && go test ./cmd/... -run TestServe -v`
Expected: 相关 serve 测试 PASS。

- [ ] **Step 7: 手动验证 CLI flag 生效**

```bash
cd ~/vscode/mengbin/gobin && go run ./cmd/gobin --help | grep -E 'verbose|log-format|log-file'
```
Expected: 三个全局 flag 出现在 Global Flags 段。

```bash
cd ~/vscode/mengbin/gobin && go run ./cmd/gobin serve --help | grep -c '\-\-verbose'
```
Expected: `1`（来自全局 persistent flag，serve 不再有自己的）。

- [ ] **Step 8: 提交**

```bash
cd ~/vscode/mengbin/gobin
git add cmd/gobin/commands/logging.go cmd/gobin/main.go cmd/gobin/commands/serve.go cmd/gobin/commands/serve_watcher.go
git commit -m "feat(log): wire global --verbose/--log-format/--log-file flags and unify verbose"
```

---

## Task 6: build.go 改造（分离用户输出与诊断日志）

**Files:**
- Modify: `cmd/gobin/commands/build.go`

- [ ] **Step 1: 添加 import 与组件 logger，埋点 runBuild**

把 `cmd/gobin/commands/build.go` 顶部 import 块改为：

```go
import (
	"fmt"
	"io"
	"time"

	"github.com/mengbin92/gobin/internal/generator"
	"github.com/mengbin92/gobin/internal/log"
	"github.com/spf13/cobra"
)
```

把 `runBuild` 函数体替换为（用户可见的 `fmt.Fprintf` 全部保留，新增 slog 诊断）：

```go
func runBuild(stdout io.Writer, minify bool, buildDrafts bool, cleanOutput bool, incremental bool, jobs int) error {
	logger := log.GetDefault()
	start := time.Now()

	fmt.Fprintf(stdout, "Blog Static Site Generator v%s\n", Version)
	fmt.Fprintln(stdout, "===================================")

	if incremental && cleanOutput {
		return fmt.Errorf("--incremental cannot be combined with --clean=true; pass --clean=false")
	}

	logger.Info("site build started",
		"version", Version,
		"minify", minify,
		"build_drafts", buildDrafts,
		"clean_output", cleanOutput,
		"incremental", incremental,
		"jobs", jobs,
	)

	logger.Debug("loading site build input")
	input, err := loadSiteBuildInput()
	if err != nil {
		logger.Error("failed to load site build input", "error", err)
		return err
	}

	logger.Info("content parsed", "posts", len(input.posts), "pages", len(input.pages))
	fmt.Fprintf(stdout, "Found %d posts\n", len(input.posts))

	result, err := generateSiteWithOptions(input, generator.GenerationOptions{
		OutputDir:   input.cfg.PublishDir,
		Minify:      minify,
		BuildDrafts: buildDrafts,
		CleanOutput: cleanOutput,
		Incremental: incremental,
		Concurrency: jobs,
	})
	if err != nil {
		logger.Error("site generation failed", "error", err, "elapsed", time.Since(start))
		return err
	}
	printStaticAssetStats(stdout, result)

	logger.Info("site build completed",
		"pages_rendered", result.Pages.Rendered,
		"pages_skipped", result.Pages.Skipped,
		"artifacts_ran", result.Artifacts.Ran,
		"assets_copied", result.StaticAssets.Copied,
		"publish_dir", input.cfg.PublishDir,
		"elapsed", time.Since(start),
	)

	if minify {
		fmt.Fprintf(stdout, "Site generated and minified successfully in '%s' directory\n", input.cfg.PublishDir)
	} else {
		fmt.Fprintf(stdout, "Site generated successfully in '%s' directory\n", input.cfg.PublishDir)
	}

	return nil
}
```

> 注意：`input.pages` 字段必须存在于 `siteBuildInput`。实现前用 `grep -n "pages" cmd/gobin/commands/site_ops.go cmd/gobin/commands/build.go` 确认字段名；若实际字段名不同（如 `standalonePages`），相应调整 `"pages", len(input.<field>)`。

- [ ] **Step 2: 运行构建与现有 build 测试**

Run: `cd ~/vscode/mengbin/gobin && go build ./... && go test ./cmd/... -run 'Build|Default' -v`
Expected: PASS（现有测试断言 stdout，不受 stderr 日志影响）。

- [ ] **Step 3: 手动验证输出分离**

```bash
cd ~/vscode/mengbin/gobin/example-site 2>/dev/null && go run ../cmd/gobin build --verbose 2>/tmp/build-stderr.log 1>/tmp/build-stdout.log; \
echo "--- STDOUT (用户输出) ---"; cat /tmp/build-stdout.log; \
echo "--- STDERR (诊断日志) ---"; cat /tmp/build-stderr.log
```
Expected: stdout 只含用户行（版本号/Found N posts/统计/成功）；stderr 含 `level=INFO/DEBUG ... component=...` 行。
（若无 `example-site` 可构建，改在仓库根目录运行 `go run ./cmd/gobin build --verbose` 并人工核对两路输出。）

- [ ] **Step 4: 提交**

```bash
cd ~/vscode/mengbin/gobin
git add cmd/gobin/commands/build.go
git commit -m "feat(log): separate user output from diagnostic logging in build"
```

---

## Task 7: serve 系列改造（serve.go / serve_server.go / serve_watcher.go）

**Files:**
- Modify: `cmd/gobin/commands/serve.go`
- Modify: `cmd/gobin/commands/serve_server.go`
- Modify: `cmd/gobin/commands/serve_watcher.go`

- [ ] **Step 1: serve_server.go — 替换 log.Printf**

在 `cmd/gobin/commands/serve_server.go`：

1. import 块去掉 `"log"`，加上 `"github.com/mengbin92/gobin/internal/log"`。最终 import：

```go
import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mengbin92/gobin/internal/config"
	"github.com/mengbin92/gobin/internal/log"
)
```

2. 把 `runServerLifecycle` 中（原 serve_server.go:72）：

```go
		if err := server.Shutdown(ctx); err != nil {
			log.Printf("Server forced to shutdown: %v", err)
		}
```

改为：

```go
		if err := server.Shutdown(ctx); err != nil {
			log.Error("server forced to shutdown", "error", err)
		}
```

3. 在 `runServerLifecycle` 进入后、`server.ListenAndServe()` 启动前，于 `fmt.Fprintln(stdout, "Press Ctrl+C to stop")` 之后追加一行诊断日志：

```go
	log.Info("development server listening", "addr", addr)
```

- [ ] **Step 2: serve_watcher.go — warning 改 slog，保留 verbose 文本**

在 `cmd/gobin/commands/serve_watcher.go`：

1. import 块加入 `"github.com/mengbin92/gobin/internal/log"`（若尚未导入）。

2. 把两处「无法监听目录」的警告（原 :133-135 与 :149-151）由 verbose-gated 的 stdout 改为始终走 `log.Warn`。即把：

```go
			if runtime.verbose {
				fmt.Fprintf(runtime.stdout, "Warning: Could not watch %s: %v\n", dir, err)
			}
```

替换为（两处相同）：

```go
			log.Warn("could not watch directory", "dir", dir, "error", err)
```

3. 把新目录监听失败（原 :214）：

```go
		fmt.Fprintf(runtime.stderr, "Warning: Could not watch new directory %s: %v\n", event.Name, err)
```

改为：

```go
		log.Warn("could not watch new directory", "dir", event.Name, "error", err)
```

4. 把 watcher 致命错误（原 :65, :71, :176）由 `fmt.Fprintf(runtime.stderr, ...)` 改为 `log.Error`：

- 原 :65 `fmt.Fprintf(runtime.stderr, "Failed to create file watcher: %v\n", err)` →
  `log.Error("failed to create file watcher", "error", err)`
- 原 :71 `fmt.Fprintf(runtime.stderr, "Failed to register watch paths: %v\n", err)` →
  `log.Error("failed to register watch paths", "error", err)`
- 原 :176 `fmt.Fprintf(runtime.stderr, "Watcher error: %v\n", err)` →
  `log.Error("watcher error", "error", err)`

5. **保留**这些 verbose 文本输出（属用户可见交互信息，不动）：原 :75「Watching for file changes...」、:106「Full reload」、:109「Partial rebuild」、:194「File changed」。

> 注意：若 `serve_test.go` 断言了 watcher 的 stderr 警告文本（如 "Could not watch"），这些断言需改为不检查 stderr 文本，或改为接受空 stderr。Step 4 会跑测试暴露此情况。

- [ ] **Step 3: serve.go — 构建过程诊断日志**

在 `cmd/gobin/commands/serve.go`：

1. import 块加入 `"github.com/mengbin92/gobin/internal/log"`。

2. 在 `runServeWithOps` 中，把 `fmt.Fprintln(stdout, "Building site...")` 之后追加：

```go
	log.Info("dev server build started", "drafts", buildDrafts, "watch", watch, "clean", serveClean)
```

并在 `fmt.Fprintln(stdout, "Site built successfully!")` 之前追加：

```go
	log.Info("dev server initial build completed",
		"pages_rendered", result.Pages.Rendered,
		"assets_copied", result.StaticAssets.Copied,
	)
```

- [ ] **Step 4: 构建与 serve 测试**

Run: `cd ~/vscode/mengbin/gobin && go build ./... && go vet ./... && go test ./cmd/... -run TestServe -v`
Expected: PASS。若某测试断言 watcher 警告/错误的 stderr 文本失败，按 Step 2 注释调整该断言（警告现走 slog 默认 stderr，不再经 `runtime.stderr`/`runtime.stdout`）。

- [ ] **Step 5: 提交**

```bash
cd ~/vscode/mengbin/gobin
git add cmd/gobin/commands/serve.go cmd/gobin/commands/serve_server.go cmd/gobin/commands/serve_watcher.go
git commit -m "feat(log): route serve diagnostics through slog, replace log.Printf"
```

---

## Task 8: check.go / init.go / new.go 改造

**Files:**
- Modify: `cmd/gobin/commands/check.go`
- Modify: `cmd/gobin/commands/init.go`
- Modify: `cmd/gobin/commands/new.go`

- [ ] **Step 1: check.go 埋点**

在 `cmd/gobin/commands/check.go`：

1. import 块加入 `"github.com/mengbin92/gobin/internal/log"`。

2. 在 `runCheck` 开头 `fmt.Fprintln(stdout, "Checking site...")` 之后追加：

```go
	log.Info("site check started", "include_drafts", includeDrafts)
```

3. 在每个 `[FAIL]` 分支的 `return errors.New("check failed")` 之前，对应追加一行 `log.Error`。例如 config 分支：

```go
	cfg, err := config.LoadDefault()
	if err != nil {
		log.Error("check failed: config", "error", err)
		fmt.Fprintf(stderr, "  [FAIL] config: %v\n", err)
		return errors.New("check failed")
	}
```

对其余 `[FAIL]` 分支（shortcodes、posts，以及 check.go 后半段的 pages / templates / collisions 等所有 `[FAIL]` 点）按同样模式，在 `fmt.Fprintf(stderr, "  [FAIL] <name>: %v\n", err)` 上一行加入 `log.Error("check failed: <name>", "error", err)`。`[OK]`/`[FAIL]` 的 stdout/stderr 文本全部保留。

4. 在函数成功返回 `nil` 之前追加：

```go
	log.Info("site check passed")
```

- [ ] **Step 2: init.go / new.go 埋点**

在 `cmd/gobin/commands/init.go` 与 `cmd/gobin/commands/new.go`：

1. 各自 import 块加入 `"github.com/mengbin92/gobin/internal/log"`。

2. 在各命令的主执行函数入口加入一条 INFO，记录关键参数。先用以下命令确认函数名与可用变量：

```bash
cd ~/vscode/mengbin/gobin && grep -n '^func run\|RunE' cmd/gobin/commands/init.go cmd/gobin/commands/new.go
```

3. 在 `init` 的主函数体首行（在任何 `fmt.Fprintf(stdout, ...)` 之后）加入：

```go
	log.Info("scaffolding new site")
```

在 `new` 的主函数体首行加入（若该函数有表示标题/路径的参数 `title`/`path`，带上；否则仅消息）：

```go
	log.Info("scaffolding new content")
```

4. 各自主函数中已有的错误 `return` 处（非用户校验类、属真实失败），在 return 前加 `log.Error("<action> failed", "error", err)`。保留所有 stdout 用户输出。

- [ ] **Step 3: 构建与测试**

Run: `cd ~/vscode/mengbin/gobin && go build ./... && go vet ./... && go test ./cmd/... -run 'Check|Init|New' -v`
Expected: PASS（断言 stdout 的现有测试不受影响）。

- [ ] **Step 4: 提交**

```bash
cd ~/vscode/mengbin/gobin
git add cmd/gobin/commands/check.go cmd/gobin/commands/init.go cmd/gobin/commands/new.go
git commit -m "feat(log): add diagnostic logging to check/init/new commands"
```

---

## Task 9: generator 层埋点

**Files:**
- Modify: `internal/generator/generator.go`

- [ ] **Step 1: 在 GenerateWithOptions 埋点**

在 `internal/generator/generator.go`：

1. import 块加入 `"github.com/mengbin92/gobin/internal/log"`。最终 import：

```go
import (
	"github.com/mengbin92/gobin/internal/config"
	"github.com/mengbin92/gobin/internal/log"
	"github.com/mengbin92/gobin/internal/parser"
)
```

2. 把 `GenerateWithOptions` 函数体替换为：

```go
func GenerateWithOptions(posts []*parser.Post, standalonePages []*parser.Page, cfg *config.Config, opts GenerationOptions) (*GenerationResult, error) {
	logger := log.GetDefault().With("component", "generator")
	logger.Debug("preparing generation plan",
		"posts", len(posts),
		"pages", len(standalonePages),
		"output_dir", opts.OutputDir,
		"incremental", opts.Incremental,
		"concurrency", opts.Concurrency,
	)

	plan, err := prepareGenerationPlan(posts, standalonePages, cfg, opts.OutputDir, opts.Minify, opts.BuildDrafts, opts.Incremental, opts.Concurrency)
	if err != nil {
		logger.Error("failed to prepare generation plan", "error", err)
		return nil, err
	}

	result, err := plan.ExecuteResult(opts.CleanOutput)
	if err != nil {
		logger.Error("failed to execute generation plan", "error", err)
		return nil, err
	}
	logger.Debug("generation plan executed",
		"pages_rendered", result.Pages.Rendered,
		"pages_skipped", result.Pages.Skipped,
	)
	return result, nil
}
```

- [ ] **Step 2: 构建与 generator 测试**

Run: `cd ~/vscode/mengbin/gobin && go build ./... && go test ./internal/generator/ -v 2>&1 | tail -20`
Expected: PASS。

- [ ] **Step 3: 提交**

```bash
cd ~/vscode/mengbin/gobin
git add internal/generator/generator.go
git commit -m "feat(log): instrument generator entry path"
```

---

## Task 10: parser 层埋点

**Files:**
- Modify: `internal/parser/parser.go`

- [ ] **Step 1: 在 ParsePostsWithOptions 埋点**

在 `internal/parser/parser.go`：

1. import 块加入 `"github.com/mengbin92/gobin/internal/log"`（按字母序放在 shortcode/textutil 之前的本地 import 区）。

2. 把 `ParsePostsWithOptions` 函数体替换为：

```go
func ParsePostsWithOptions(dir string, opts RenderOptions) ([]*Post, error) {
	logger := log.GetDefault().With("component", "parser")
	if dir == "" {
		return nil, nil
	}
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		logger.Debug("content directory does not exist, skipping", "dir", dir)
		return nil, nil
	}

	logger.Debug("scanning content directory", "dir", dir)

	var posts []*Post

	err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}

		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if ext != ".md" && ext != ".markdown" {
			return nil
		}

		post, err := ParsePostWithOptions(path, opts)
		if err != nil {
			return fmt.Errorf("failed to parse %s: %w", path, err)
		}
		if post != nil {
			posts = append(posts, post)
		}
		return nil
	})
	if err != nil {
		logger.Error("failed to read content directory", "dir", dir, "error", err)
		return nil, fmt.Errorf("failed to read directory %s: %w", dir, err)
	}

	logger.Info("posts parsed", "dir", dir, "total", len(posts))
	return posts, nil
}
```

- [ ] **Step 2: 构建与 parser 测试**

Run: `cd ~/vscode/mengbin/gobin && go build ./... && go test ./internal/parser/ -v 2>&1 | tail -20`
Expected: PASS。

- [ ] **Step 3: 提交**

```bash
cd ~/vscode/mengbin/gobin
git add internal/parser/parser.go
git commit -m "feat(log): instrument parser content scanning"
```

---

## Task 11: config 层埋点

**Files:**
- Modify: `internal/config/config.go`

- [ ] **Step 1: 在 Load 埋点**

在 `internal/config/config.go`：

1. import 块加入 `"github.com/mengbin92/gobin/internal/log"`。

2. 把 `Load` 函数体替换为：

```go
func Load(path string) (*Config, error) {
	logger := log.GetDefault().With("component", "config")
	logger.Debug("loading configuration", "path", path)

	data, err := os.ReadFile(path)
	if err != nil {
		logger.Error("failed to read config file", "path", path, "error", err)
		return nil, err
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		logger.Error("failed to parse config file", "path", path, "error", err)
		return nil, err
	}

	normalized := Normalize(&cfg)
	if err := ValidateInDir(normalized, filepath.Dir(path)); err != nil {
		logger.Error("config validation failed", "path", path, "error", err)
		return nil, err
	}

	logger.Debug("config loaded", "path", path, "title", normalized.Title, "publish_dir", normalized.PublishDir)
	return normalized, nil
}
```

> 注意：`normalized.PublishDir` / `normalized.Title` 字段名需与 `Config` 结构体一致（结构体已含 `Title`；用 `grep -n 'PublishDir' internal/config/config.go` 确认 `PublishDir` 字段名，否则替换为实际字段或删去该键）。

- [ ] **Step 2: 构建与 config 测试**

Run: `cd ~/vscode/mengbin/gobin && go build ./... && go test ./internal/config/ -v 2>&1 | tail -20`
Expected: PASS。

- [ ] **Step 3: 提交**

```bash
cd ~/vscode/mengbin/gobin
git add internal/config/config.go
git commit -m "feat(log): instrument config loading and validation"
```

---

## Task 12: 全量验收

**Files:** 无（验证 + 文档）

- [ ] **Step 1: 全量构建 / vet / 测试**

Run: `cd ~/vscode/mengbin/gobin && go build ./... && go vet ./... && go test ./... 2>&1 | tail -30`
Expected: 全部 PASS（验收标准 1、2）。

- [ ] **Step 2: 验收 — 默认输出与改造前一致**

```bash
cd ~/vscode/mengbin/gobin && go run ./cmd/gobin build 2>/dev/null
```
Expected: stdout 仅含用户行（版本号 / Found N posts / Pages/Artifacts/Static 统计 / 成功提示），无 `level=` 日志行（验收标准 3）。

- [ ] **Step 3: 验收 — verbose / json / file**

```bash
cd ~/vscode/mengbin/gobin
echo "--- verbose ---"; go run ./cmd/gobin build --verbose 2>&1 >/dev/null | grep -E 'level=(DEBUG|INFO)' | head
echo "--- json ---"; go run ./cmd/gobin build --log-format json 2>/tmp/g.json >/dev/null; head -1 /tmp/g.json | python3 -m json.tool >/dev/null && echo "valid JSON"
echo "--- file ---"; go run ./cmd/gobin build --log-file /tmp/g.log 2>/dev/null >/dev/null; test -s /tmp/g.log && echo "file written"
```
Expected: verbose 显示 DEBUG/INFO 行且含 `component=`；json 首行为合法 JSON；file 非空（验收标准 4、5、6）。

- [ ] **Step 4: 验收 — serve -v 行为**

```bash
cd ~/vscode/mengbin/gobin && go run ./cmd/gobin serve --help | grep -c '\-\-verbose'
```
Expected: `1`（全局 flag 唯一来源，serve 无重复定义；验收标准 7）。
（如需运行时验证 watcher verbose 文本，手动 `go run ./cmd/gobin serve -v` 触发文件变更观察 "File changed" 行，Ctrl+C 退出。）

- [ ] **Step 5: 更新文档并提交**

将设计文件状态从「待实现」更新为「已实现」。

```bash
cd ~/vscode/mengbin/gobin
# 在 docs/superpowers/specs/2026-06-02-logging-system-design.md 顶部把 "状态：已批准，待实现" 改为 "状态：已实现 (2026-06-02)"
git add docs/superpowers/specs/2026-06-02-logging-system-design.md
git commit -m "docs(logging): mark logging system spec as implemented"
```

---

## Self-Review 结果

- **Spec 覆盖**：§3 包结构 → Task 1-4；§5 CLI/verbose 统一 → Task 5；§6 commands → Task 6-8；§7 internal → Task 9-11；§9 测试策略 → Task 1-4 单测 + 各任务回归；§10 验收标准 1-7 → Task 12 Step 1-4。
- **占位符扫描**：无 TBD/TODO；所有代码步骤含完整代码。少量「实现前用 grep 确认字段名」的核对点（`input.pages`、`PublishDir`）是真实存在的代码事实校验，非占位。
- **类型一致性**：`Options`/`Level`/`Format`/`New`/`NewFromCLI`/`WithComponent`/`SetDefault`/`GetDefault`/`resolveWriter`/`parseLevel`/`FromContext`/`IntoContext` 跨 Task 命名一致；commands 包 `verbose`/`logFormat`/`logFile`/`AddGlobalFlags`/`InitLogging` 在 Task 5 定义并被 main.go 引用，命名一致。
