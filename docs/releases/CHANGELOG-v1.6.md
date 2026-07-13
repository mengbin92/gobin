# Gobin v1.6.0 更新日志

## 发布日期 - 2026-07-04

Gobin v1.6.0 是一次面向解析阶段性能的功能版本。性能画像（基于真实博客 610 篇 + pprof CPU profile）确认 Markdown 解析占全量构建 ~64% 时间，是唯一的实质热点。v1.6.0 把解析阶段改为可选并行，复用 v1.4.0 的 `--jobs` flag 与 `min(NumCPU, 4)` 自动封顶。真实 610 篇端到端 clean build **305 ms → 207 ms（1.47x 加速）**，HTML/资产产物字节级一致。本次发布保持配置、模板、CLI 入口、增量构建、并行渲染、serve partial rebuild 既有行为完全向后兼容。

---

## 新增功能

### Markdown 解析并行化

- 解析阶段（`parser.ParsePostsWithOptions` / `ParsePagesWithOptions`）改为 collect-then-parse 模型：`filepath.WalkDir` 先收集 `.md`/`.markdown` 文件列表，再交给 worker 池并行解析。
- 并发度跟随现有 `--jobs` flag，与页面渲染共用同一并发度参数：用户只需记一个 flag。
- 默认并行度与 `autoConcurrencyCap` 一致：`min(NumCPU, 4)`；`--jobs 1` 强制串行；显式 `--jobs N` 不封顶。
- 解析结果按文件路径字典序回填，与 `filepath.WalkDir` 串行顺序一致；下游 `prepareContentPlan` 按日期重排、`contentCache.assemble` 按 FilePath 排序，产物字节级一致。
- 错误传播用 `sync.Once` + `atomic.Bool` 捕获首个错误，其余 worker 尽快退出，与 `renderPageSpecsConcurrent` 模式一致。
- 新增公开 API：`ParsePostsWithOptionsConcurrent` / `ParsePagesWithOptionsConcurrent(dir, opts, concurrency int)`。旧函数 `ParsePostsWithOptions` / `ParsePagesWithOptions` 签名不变，内部默认 `concurrency=0`（auto）。
- 单文件场景自动退化到串行路径，无 worker 调度开销。
- 接线：`loadSiteBuildInput` 内部用 auto；新增 `loadSiteBuildInputWithConcurrency(con)`，`build` 命令把 `--jobs` 透传到解析阶段；`check` 走 auto。

### 并发安全验证

- `ParsePostWithOptions` / `ParsePageWithOptions` 是纯函数（`os.ReadFile` → `splitFrontMatter` → `yaml.Unmarshal` → `renderMarkdownWithOptions` → `normalizeXxxFrontMatter`），无包级共享可变状态。
- `renderMarkdownWithOptions` 每次 `goldmark.New`，goldmark 实例不跨调用共享。
- `shortcode.Registry` 在 `LoadRegistry` 构造后只读：`templates map[string]*template.Template` 不再写入；`Lookup` 与 `template.Execute` 并发安全（Go 标准库保证）。
- worker 各写入预分配 slice 的独立索引槽（stripe 分配），无写竞争。
- `go test -race ./...` 全绿。

---

## 改进

- 解析阶段从 610 篇 61.7 ms 降到 26.4 ms（`--jobs 4`），加速比 2.34x；端到端 1.47x 加速。
- 用户体验：`--jobs` 现在控制解析 + 渲染两个阶段，不需要额外 flag。
- `internal/parser/parallel.go` 抽出 `parsePostFilesConcurrent` / `parsePageFilesConcurrent` helper，结构与 `renderPageSpecsConcurrent` 对称，便于维护。

---

## 兼容性

- 本版本保持配置、模板、CLI 入口、增量构建、并行渲染、serve partial rebuild 完全向后兼容。
- `ParsePosts` / `ParsePages` / `ParsePostsWithOptions` / `ParsePagesWithOptions` 签名与行为不变（默认 auto 并发，单文件退化为串行）。
- `RenderOptions` 不新增字段，env hash 不变，增量构建 cache 兼容性保持。
- `GenerationOptions.Concurrency` 字段已存在（v1.4），本次让它也作用于解析阶段，文档同步更新。
- `--jobs` 语义从"页面渲染 worker 数"扩展为"解析与渲染 worker 数"；用户旧习惯（`--jobs 4`）直接受益。
- 增量构建 manifest 的 source_hash 基于文件内容，与解析顺序无关，无失效风险。
- 库 API：新增 `ParsePostsWithOptionsConcurrent` / `ParsePagesWithOptionsConcurrent`；`loadSiteBuildInputWithConcurrency` 取代裸 `loadSiteBuildInput` 在需要显式并发度的代码路径上使用；其余签名不变。

---

## 性能

性能画像（基于真实博客 610 篇，平均 7KB/篇）：

**解析阶段隔离基准**（`BenchmarkParsePosts_Concurrency`，M5 Pro）：

| 文章数 | jobs=1 | jobs=4 | 加速比 |
| --- | --- | --- | --- |
| 100 | 1.99 ms | 1.32 ms | 1.51x |
| 500 | 11.68 ms | 7.27 ms | 1.61x |

**端到端 clean build**（610 篇真实博客）：

| --jobs | 端到端耗时 | 加速比 |
| --- | --- | --- |
| 1（串行） | 305 ms | 1.00x |
| 4 | 207 ms | 1.47x |
| 8 | 207 ms | 1.47x |
| 0（auto） | 209 ms | 1.46x |

> 解析阶段加速比 2.34x（隔离基准）→ 端到端 1.47x；差额来自 `os.RemoveAll` clean 阶段（占 ~38%）、页面渲染 I/O、聚合产物等未被并行的固定开销。

**产物一致性验证**：`gobin build --jobs 1` 与 `gobin build --jobs 4` 同一站点 730 个产物，`diff -r` 仅 `index.xml` / `index.atom` 的 RFC1123/RFC3339 时间戳差异（与并发无关，串行两次构建也会有）。

---

## 验证

发布前执行：

```bash
go test ./...
go test -race ./internal/parser/... ./internal/generator/... ./cmd/gobin/commands/...
go vet ./...
gofmt -l $(find . -name '*.go' -not -path './website/*' -not -path './public/*')
make test-coverage
make release-local
```

并手测：

- 真实 610 篇站点 `gobin build --jobs 1` 与 `gobin build --jobs 4`，`diff -r public/` 仅时间戳差异 → 产物字节级一致。
- `gobin build --jobs 1 --drafts` 与 `gobin build --jobs 4 --drafts` 在 610 篇站点上跑三次取中位数：jobs=1 ~305 ms，jobs=4 ~207 ms（1.47x）。
- `gobin check --drafts` 在 610 篇站点上正常返回 `[OK] parsed 610 post(s) from _posts`。
- 单文件站点 `gobin build --jobs 4` 走串行 fallback，无 worker 调度开销。
- 含损坏 front matter 的文件（缺闭合 `---`）：`gobin build` 返回错误并包含文件路径，watch 模式 partial rebuild 不会让部分 post 被错误地保留。
- 增量构建：编辑 1 篇 post → 解析仅 reparse 该 1 篇（serve `contentCache` 行为不变），build 端到端走 `--incremental` 跳过未变页面与聚合产物。
