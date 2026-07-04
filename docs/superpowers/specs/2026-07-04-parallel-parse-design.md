# Markdown 解析并行化 — 实现规格 (Spec)

> 日期：2026-07-04
> 状态：草稿，待 review
> 范围：v1.6.0
> 承接：性能画像报告（2026-07-04，基于真实博客 610 篇实测）

## 1. 问题

性能画像（CPU profile + 隔离 benchmark）确认 Markdown 解析是全量构建的**唯一实质热点**：

- 610 篇真实内容，串行解析 61.7 ms，占 clean build 总耗时（96.6 ms）的 ~64%
- 纯 CPU 计算（goldmark 渲染 + 模板执行 + 序列化）合计 < 10%，构建整体 I/O-bound
- `parser.ParsePostsWithOptions` / `ParsePagesWithOptions` 当前是 `filepath.WalkDir` 串行循环，每个文件 `os.ReadFile` + goldmark `Convert`

对照基准（解析阶段隔离，610 篇）：

| 方案 | 耗时 | 加速比 |
|------|------|--------|
| 串行（当前） | 61.7 ms | 1.00x |
| 并行 w=2 | 37.1 ms | 1.66x |
| 并行 w=4 | 26.4 ms | 2.34x |
| 并行 w=8 | 23.4 ms | 2.63x |
| 并行 w=16 | 23.0 ms | 2.69x（封顶） |

端到端预期：clean build 96.6 ms → ~61 ms（**1.58x 加速**）。

聚合产物并行化经实测仅占 2%（1.8 ms），不在本次范围。

## 2. 目标

v1.6.0 把 Markdown 解析改为可选并行，复用 v1.4.0 页面渲染的 `--jobs` 语义与 `normalizeConcurrency` / `autoConcurrencyCap` 基础设施：

- `ParsePostsWithOptions` / `ParsePagesWithOptions` 在文件数 > 1 时用 worker 池并行解析
- 与串行产物**字节级一致**（下游 generator 按日期重排、site_cache 按 FilePath 排序，不依赖解析顺序）
- 与 `--jobs` flag 正交：解析并行度跟随 `GenerationOptions.Concurrency`
- `serve --watch` 的 partial rebuild 已有 `contentCache`，只 reparse 变化文件，本次不改动其语义；但 full reload（首启 / structural change）自动受益

## 3. 设计决策

| 决策 | 选择 | 理由 |
|------|------|------|
| 并行度来源 | `GenerationOptions.Concurrency`（`--jobs`） | 与页面渲染统一，用户只需记一个 flag；画像显示解析与渲染的 sweet spot 都在 4 左右 |
| 默认并行度 | `min(NumCPU, 4)` | 与 `autoConcurrencyCap` 一致；画像显示 w=8 之后收益封顶 |
| 触发条件 | 文件数 > 1 且 concurrency > 1 | 单文件无并行必要；与 `renderPageSpecsConcurrent` 的 fallback 一致 |
| 结果顺序 | 按 path 字典序回填 | `filepath.WalkDir` 本身按字典序遍历；并行后必须显式排序以保持与串行字节一致，避免引入不可复现 diff |
| 错误传播 | 首个错误（`sync.Once` + atomic flag） | 与 `renderPageSpecsConcurrent` 一致；解析错误应中断而非部分成功 |
| goldmark 实例 | 每个 goroutine 独立 `goldmark.New` | 当前 `renderMarkdownWithOptions` 每次 new，无共享；保持现状，避免引入跨 goroutine 共享 |
| shortcode.Registry | 共享只读 | `Registry.templates` 是构造后只读 map，`Lookup` + `template.Execute` 并发安全；与页面渲染共享同一 registry 的模式一致 |
| API 兼容 | `ParsePostsWithOptions` / `ParsePagesWithOptions` 签名不变 | 新增内部 `parseDirConcurrent` helper；公开函数默认走自动并发，串行入口 `ParsePosts`/`ParsePages` 保持旧行为 |

## 4. 并发安全分析

### 4.1 无共享可变状态

- `ParsePostWithOptions(path, opts)` / `ParsePageWithOptions(path, baseDir, opts)` 是纯函数：`os.ReadFile` → `splitFrontMatter` → `yaml.Unmarshal` → `renderMarkdownWithOptions` → `normalizePostFrontMatter`，全部基于入参，无包级可变状态
- `renderMarkdownWithOptions` 每次 `goldmark.New(options...)`，goldmark 实例不跨调用共享
- `normalizePostFrontMatter` / `normalizePageFrontMatter` 纯函数，只操作入参副本

### 4.2 shortcode.Registry 只读

`RenderOptions.Shortcodes *shortcode.Registry` 在解析期间被多 goroutine 共享，但：

- `Registry.templates map[string]*template.Template` 在 `LoadRegistry` 构造后不再写入
- `Registry.Lookup` 只读 map
- `Registry.Render` → `expandPre` → `renderNode` → `tmpl.Execute`，`*template.Template` 的 `Execute` 并发安全（Go 标准库保证）
- 与页面渲染路径（`renderPageSpecsConcurrent` 共享同一 `*template.Template`）模式一致，已通过 `go test -race`

### 4.3 结果回填

worker 各自写入预分配 `[]*Post`/`[]*Page` 的独立索引槽（stripe 分配），无写竞争；最终按 path 字典序重排。

## 5. 实现方案

### 5.1 新增 helper：`parseFilesConcurrent`

`internal/parser/parser.go` 新增内部泛型 helper（用类型参数或两份近似实现，倾向两份以保持简单可读）：

```go
// parsePostFilesConcurrent parses files in parallel and returns posts sorted
// by path to match filepath.WalkDir's lexical order.
func parsePostFilesConcurrent(files []string, opts RenderOptions, concurrency int) ([]*Post, error)
func parsePageFilesConcurrent(files []string, baseDir string, opts RenderOptions, concurrency int) ([]*Page, error)
```

结构（与 `renderPageSpecsConcurrent` 对称）：

- `concurrency <= 1 || len(files) <= 1` → 串行 fallback
- stripe 分配：worker w 处理 `files[w], files[w+workers], ...`
- 每个 worker 独立 `ParsePostWithOptions`，结果写入预分配 slice 的对应槽
- `sync.Once` + `atomic.Bool` 捕获首个错误，其余 worker 尽快退出
- 完成后按 `files` 原序（字典序，因 WalkDir 已排序）回填，保证与串行一致

### 5.2 改造 `ParsePostsWithOptions` / `ParsePagesWithOptions`

两阶段：

1. `filepath.WalkDir` 收集 `.md`/`.markdown` 文件列表（保持原过滤逻辑）
2. 调 `parsePostFilesConcurrent(files, opts, concurrency)` / `parsePageFilesConcurrent`

### 5.3 并发度来源

`ParsePostsWithOptions` 当前签名无 concurrency 参数。两个选择：

- **A. 新增 `ParsePostsWithOptionsConcurrent(dir, opts, concurrency)`**，旧函数默认 concurrency=0（auto）
- **B. 在 `RenderOptions` 加 `Concurrency int` 字段**

选 **A**：`RenderOptions` 语义是"Markdown 渲染选项"（unsafe HTML、shortcodes），塞并发度会污染其语义且影响 env hash（shortcode registry 已刻意排除出 JSON）。新增 `*Concurrent` 变体，旧函数转发并传 `0`（auto），保持向后兼容。

### 5.4 CLI 接线

`cmd/gobin/commands/site_ops.go` 的 `loadSiteBuildInput` 改用 `ParsePostsWithOptionsConcurrent` / `ParsePagesWithOptionsConcurrent`，并发度从 `GenerationOptions.Concurrency` 取（build 命令已有 `--jobs`）。

`serve` 路径：

- `initialBuild` 与 `rebuild` 的 `loadSiteBuildInput` 走并发版（auto）
- `serve_watcher` 的 `contentCache` partial rebuild 只 reparse 单文件，无并行必要，不改
- `serve_runtime` 的 `serveBuilder` 把 `Concurrency` 传入 `GenerationOptions`（serve 目前没暴露 `--jobs`，默认 auto 即可）

## 6. 产物一致性

- 并行解析结果按 path 字典序回填，与串行 `filepath.WalkDir` 顺序一致
- `generator.prepareContentPlan` 对 posts 按 `Date` 非稳定排序（`sort.Slice`），解析顺序不影响最终产物
- `contentCache.assemble` 已按 FilePath 字典序输出
- 增量构建 manifest 的 source_hash 基于文件内容，与解析顺序无关
- 预期：`gobin build --jobs 1` 与 `gobin build --jobs 4` 产物 `diff -r` 无差异（同 v1.4 并行渲染的验证标准）

## 7. 测试方案

### 7.1 单元测试（`internal/parser/parser_test.go`）

- `TestParsePostsWithOptionsConcurrent_MatchesSerial`：多文件（含嵌套目录、.md/.markdown 混合、草稿），断言并发结果与串行逐字段相等
- `TestParsePostsWithOptionsConcurrent_PropagatesError`：混入一个 front matter 损坏文件，断言返回错误且错误信息含路径
- `TestParsePostsWithOptionsConcurrent_PreservesOrder`：断言结果按 path 字典序
- `TestParsePagesWithOptionsConcurrent_*`：对称覆盖

### 7.2 集成测试（`internal/generator/generator_test.go`）

- `TestGenerateWithOptions_ConcurrentParse_MatchesSerial`：同一站点 `Concurrency=1` vs `Concurrency=4`，`diff -r` 输出目录无差异

### 7.3 基准测试（`internal/generator/incremental_bench_test.go`）

新增 `BenchmarkParsePosts_Concurrency`，复用 `generateIncrementalBenchmarkPosts` 的 fixture：

```
BenchmarkParsePosts_Concurrency/posts=100/jobs=1
BenchmarkParsePosts_Concurrency/posts=100/jobs=4
BenchmarkParsePosts_Concurrency/posts=500/jobs=1
BenchmarkParsePosts_Concurrency/posts=500/jobs=4
```

### 7.4 race 检测

`go test -race ./internal/parser/... ./internal/generator/... ./cmd/gobin/commands/...` 必须通过。

## 8. 兼容性

- `ParsePosts` / `ParsePages` / `ParsePostsWithOptions` / `ParsePagesWithOptions` 签名与行为不变（默认 auto 并发，单文件退化为串行）
- `RenderOptions` 不新增字段，env hash 不变
- `GenerationOptions.Concurrency` 字段已存在（v1.4），本次仅让它作用于解析阶段
- `--jobs` flag 语义从"页面渲染 worker 数"扩展为"解析与渲染 worker 数"，文档更新说明
- 增量构建、serve partial rebuild、shortcode、主题系统均不受影响

## 9. 验收标准

- [ ] `go test -race ./...` 通过
- [ ] `gobin build --jobs 1` 与 `gobin build --jobs 4` 产物 `diff -r` 无差异
- [ ] `BenchmarkParsePosts_Concurrency/posts=500/jobs=4` 相对 `jobs=1` 有 ≥ 1.5x 加速
- [ ] 真实 610 篇 clean build 端到端 ≥ 1.3x 加速
- [ ] gofmt + go vet 通过
