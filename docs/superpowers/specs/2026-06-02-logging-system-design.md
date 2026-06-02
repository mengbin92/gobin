# Gobin 统一日志系统 — 实现规格 (Spec)

> 日期：2026-06-02
> 状态：已实现 (2026-06-02)
> 基础设计：`~/Desktop/logging-system-design.md`

---

## 1. 目标与范围

为 Gobin（Go 1.25 静态博客生成器）引入统一的、基于标准库 `log/slog` 的结构化日志系统，
替换当前散落在 commands 层的 ~40 处 `fmt.Fprintf` 诊断输出与唯一一处 `log.Printf`，
并为 internal 层（generator/parser/config）提供可观测性。

**本期范围（已确认）：Phase 1 + 2 + 3 全量。**

- Phase 1：基础设施 —— `internal/log` 包 + CLI flag + 全局 logger 初始化
- Phase 2：commands 层改造 —— 严格分离用户输出与诊断日志
- Phase 3：internal 层埋点 —— generator / parser / config

**明确排除（Phase 4，不在本期）：**
- `config.yaml` 的 `logging` 配置段与 `Config` 结构体扩展
- HTTP 访问日志中间件
- 各阶段耗时性能报告

## 2. 核心设计决策

| 决策 | 选择 | 理由 |
|------|------|------|
| 日志库 | `log/slog`（标准库） | 零外部依赖，Go 1.25 原生，结构化一等公民 |
| 注入方式 | 全局 logger + `slog.SetDefault` | 对 internal 层零侵入，不改函数签名 |
| 用户输出 vs 日志 | stdout 给用户，slog 走 stderr/文件 | POSIX 惯例，CI 管道可分离 |
| 默认级别 | INFO | 生产不刷屏，`--verbose` 一键开 DEBUG |
| 格式 | text（终端默认）/ json（CI） | 人读 / 机读 |
| 文件输出 | **追加写单文件，无滚动** | 保持零外部依赖；打开失败回退 stderr |
| verbose flag | **统一为全局 persistent flag** | 消除与 serve 局部 flag 的冲突，语义一致 |
| internal 层来源标识 | `.With("component", "...")` | 支持按组件过滤，无需改签名 |

## 3. 目录结构

```
internal/log/                 # 新增包
├── log.go                    # Options/Level/Format + New/NewFromCLI + 全局 logger + 便捷函数
├── context.go                # FromContext / IntoContext（为后续 HTTP 中间件预留）
├── writer.go                 # resolveWriter：stderr/stdout/文件追加写
└── log_test.go               # 单元测试
```

**约束**：`internal/log` 不得 import 任何其他 internal 包（config/parser/generator 等都将 import log，
反向依赖会导致 import cycle）。`log` 包只依赖标准库。

## 4. API 设计

### 4.1 `internal/log/log.go`

```go
package log

type Level string
const ( LevelDebug Level = "debug"; LevelInfo Level = "info"; LevelWarn Level = "warn"; LevelError Level = "error" )

type Format string
const ( FormatText Format = "text"; FormatJSON Format = "json" )

type Options struct {
    Level     Level
    Format    Format
    Output    string // "stderr"(默认) | "stdout" | 文件路径
    AddSource bool
}

func Default() Options                                   // INFO + text + stderr
func New(opts Options) *slog.Logger                      // 按配置构造
func NewFromCLI(verbose bool, format, logFile string) *slog.Logger
func WithComponent(l *slog.Logger, component string) *slog.Logger

// 全局 logger
func SetDefault(l *slog.Logger)   // 同时调用 slog.SetDefault
func GetDefault() *slog.Logger
func Debug/Info/Warn/Error(msg string, args ...any)      // 便捷转发到 defaultLogger
```

- `NewFromCLI`：`verbose=true` → Level=debug 且 AddSource=true；`format`/`logFile` 非空时覆盖默认。
- `New`：根据 Format 选 `slog.NewTextHandler` / `slog.NewJSONHandler`；`AddSource` 仅在 DEBUG 级别生效。
- `init()` 初始化 `defaultLogger = New(Default())`，保证未初始化时也可用。

### 4.2 `internal/log/context.go`

```go
func FromContext(ctx context.Context) *slog.Logger  // 无则返回 defaultLogger
func IntoContext(ctx context.Context, l *slog.Logger) context.Context
```

### 4.3 `internal/log/writer.go`

```go
func resolveWriter(output string) io.Writer
// "stdout" -> os.Stdout
// "stderr" | "" -> os.Stderr
// 其他 -> os.OpenFile(append|create|wronly, 0644)，失败则写一行 warning 到 stderr 并回退 stderr
```

## 5. CLI 集成（`cmd/gobin/main.go`）

```go
var ( verbose bool; logFmt string; logFile string )

func init() {
    rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable debug logging")
    rootCmd.PersistentFlags().StringVar(&logFmt, "log-format", "text", "Log format: text or json")
    rootCmd.PersistentFlags().StringVar(&logFile, "log-file", "", "Write logs to file (default: stderr)")
    rootCmd.PersistentPreRun = func(cmd *cobra.Command, args []string) {
        log.SetDefault(log.NewFromCLI(verbose, logFmt, logFile))
    }
}
```

**verbose 统一**（关键改动）：
- 删除 `serve.go:68` 的局部 `ServeCmd.Flags().BoolVarP(&serveVerbose, "verbose", "v", ...)`。
- `serveVerbose` 及 `runtime.verbose` 改为读取全局 `verbose`（`cmd/gobin/main.go` 的包变量在
  commands 包之外，需通过 serve 命令的 `PreRun`/`RunE` 内读取 `cmd.Flags().GetBool("verbose")`，
  或在 commands 包内暴露 getter）。实现时确认取值路径，保证 `serve -v` 仍触发 watcher 详细输出。
- watcher 现有的 verbose 文本输出行为保持不变（仍走 stdout，属用户可见信息）。

## 6. Commands 层改造（Phase 2）

原则：**用户面向信息走 stdout（保留 `fmt.Fprintf`），诊断信息走 slog（stderr/文件）。**

| 文件 | 改造要点 |
|------|---------|
| `build.go` | 保留版本号/统计等用户输出；新增 `Info` 构建开始/完成、`Debug` 加载配置、`Error` 失败路径 |
| `serve.go` | 构建过程诊断日志；保留启动地址等用户输出 |
| `serve_server.go` | 替换 `log.Printf("Server forced to shutdown...")` 为 `log.Error(...)` |
| `serve_watcher.go` | warning 类（如无法监听目录）改 `log.Warn`；verbose 文本输出保留为用户输出 |
| `check.go` | 校验过程 `Debug`/`Info`，失败 `Error`；校验结论（OK/FAIL）保留 stdout |
| `init.go` / `new.go` | 脚手架关键步骤 `Debug`/`Info` |

**判定准则**：用户为完成任务必须看到的 → stdout；用于排查/可在 INFO 下省略的 → slog。

## 7. Internal 层埋点（Phase 3）

各包内 `logger := log.GetDefault().With("component", "<pkg>")`，不改函数签名。

- `generator/`：生成计划准备（Debug）、计划执行（Debug：page/artifact count）、页面渲染、
  增量跳过判断、产物统计、失败路径（Error）。component=`generator`
- `parser/`：内容目录扫描（Debug）、posts 解析完成（Info：total）、malformed 跳过（Warn）、
  目录读取失败（Error）。component=`parser`
- `config/`：配置加载（Debug）、加载完成（Debug：file/theme）、校验失败（Error）、
  默认值应用。component=`config`

## 8. 分级策略

| 级别 | 场景 |
|------|------|
| DEBUG | `--verbose` 才开：模板加载、文件扫描详情、增量跳过、单页渲染耗时 |
| INFO | 关键事件：构建开始/完成、解析数量、服务器启动 |
| WARN | 可恢复：监听目录缺失、front matter 异常、模板候选降级 |
| ERROR | 不可恢复：配置解析失败、输出写入失败、渲染崩溃 |

## 9. 测试策略

- **`internal/log/log_test.go`** 覆盖：
  - `parseLevel` / Level 映射
  - text vs json handler 选择（向自定义 buffer writer 写入并断言内容）
  - `resolveWriter`：stdout/stderr/合法文件路径/非法路径回退 stderr
  - `WithComponent` 输出带 `component` 字段
  - `NewFromCLI`：verbose → debug 级别可见
- **回归**：commands/internal 改造后运行现有测试套件。现有测试断言的是 **stdout**，
  日志走 stderr，理论上不受影响。
- **测试输出污染**：全局 logger 默认 INFO → stderr。stderr 不影响 stdout 断言，
  保持默认即可（最小改动）。若发现某测试捕获 stderr 并断言，则在该测试中用
  `log.SetDefault(log.New(log.Options{Output:"stderr", Level: LevelError}))` 或 `io.Discard` 降噪。

## 10. 验收标准

1. `go build ./...` 与 `go vet ./...` 通过。
2. `go test ./...` 全绿（含现有套件）。
3. `gobin build` 默认输出与改造前用户可见内容一致（stdout 不变）。
4. `gobin build --verbose 2>&1` 显示 DEBUG/INFO slog 行（含 component 标签）。
5. `gobin build --log-format json 2>build.log` 产出合法 JSON 行日志。
6. `gobin build --log-file /tmp/g.log` 追加写入该文件；不可写路径回退 stderr 并提示。
7. `gobin serve -v` 仍触发 watcher 详细输出（verbose 统一后行为不回退）。

## 11. 实现顺序

1. `internal/log` 包 + 单测（先行，独立可测）
2. `main.go` persistent flags + PreRun + verbose 统一（删 serve 局部 flag）
3. commands 层逐文件改造（build → serve* → check → init/new）
4. internal 层埋点（generator → parser → config）
5. 全量 `go build` / `go vet` / `go test`，按验收标准核对
