# Gobin v1.6.0 发布说明

## 发布日期 - 2026-07-04

Gobin v1.6.0 是一次面向**解析阶段性能**的功能版本。本次发布在 v1.5.0 的基础上把 `--jobs` 扩展到 Markdown 解析阶段，把全量构建里唯一的实质热点（解析占 ~64%）并行化。真实 610 篇端到端 clean build **305 ms → 207 ms（1.47x 加速）**，HTML/资产产物字节级一致。

---

## 亮点

- **`--jobs` 现在同时控制解析与渲染**：`gobin build --jobs 4` 在 610 篇站点上比 `--jobs 1` 快 1.47x（端到端），解析阶段隔离加速比 2.34x。用户只需记一个 flag。
- **默认 `min(NumCPU, 4)` 自动封顶**：与 v1.4 页面渲染同 cap，避免 I/O 竞争退化。
- **完全向后兼容**：现有 `--jobs` 用法（`0`=auto / `1`=串行 / `N`=显式）直接受益，无需任何配置改动。`RenderOptions` 不新增字段，增量构建 cache 兼容性保持。

---

## 升级方式

```bash
go install github.com/mengbin92/gobin/cmd/gobin@v1.6.0
```

或从 GitHub Releases 下载对应平台的压缩包，并使用 `SHA256SUMS` 校验。

Docker 用户可以使用：

```bash
docker pull docker.io/mengbin92/gobin:v1.6.0
```

---

## Markdown 解析并行化

### 1. 解决什么问题

性能画像（基于真实博客 610 篇 + pprof CPU profile）确认：

| 阶段 | 耗时 | 占比 |
| --- | --- | --- |
| Markdown 解析 | 61.7 ms | **~64%** |
| `os.RemoveAll` cleanOutput | ~38 ms | ~39% |
| 页面渲染（已并行） | ~35 ms | ~36% |
| 聚合产物 | 1.8 ms | ~2% |
| 模板执行 | < 1 ms | < 1% |

`gobin build` 是 I/O-bound（80%+ CPU 时间在 `syscall.rawsyscalln`），但解析阶段的 I/O 与 goldmark 渲染是真正没被并行化的 O(N) 热点。聚合产物并行化经实测仅占 2%，不值得做；本次只动解析。

### 2. 怎么用

`--jobs` 的取值含义**不变**，但作用范围从"页面渲染"扩展到"解析 + 渲染"：

| 取值 | 行为 |
|------|------|
| `0`（默认） | auto = `min(NumCPU, 4)`，作用于解析与渲染两个阶段 |
| `1` | 强制串行（两个阶段都走串行） |
| `>1` | 显式 N 个 worker，**不封顶**（用户自负责任） |

```bash
# 解析 + 渲染都走 auto (min(NumCPU, 4))
gobin build

# 解析 + 渲染都用 4 worker
gobin build --jobs 4

# 全串行（CI 想避开并发）
gobin build --jobs 1
```

### 3. 并发安全

- `ParsePostWithOptions` / `ParsePageWithOptions` 是纯函数，无包级共享可变状态
- `shortcode.Registry` 在 `LoadRegistry` 构造后只读（`templates map` 不再写入）
- `goldmark.New` 每次调用构造新实例，不跨调用共享
- worker stripe 分配到预分配 slice 槽位，无写竞争
- `go test -race ./...` 全绿

### 4. 产物一致性

- 解析结果按文件路径字典序回填，与 `filepath.WalkDir` 串行顺序一致
- `generator.prepareContentPlan` 按 `Date` 重排 posts，`contentCache.assemble` 按 FilePath 排序，均不依赖解析顺序
- 增量构建 manifest 的 `source_hash` 基于文件内容，与解析顺序无关

**实测**：`gobin build --jobs 1` 与 `gobin build --jobs 4` 同一站点 730 个产物，`diff -r` 仅 `index.xml` / `index.atom` 的 RFC1123/RFC3339 时间戳差异（与并发无关，串行两次构建也会有）。

### 5. 性能数据

**端到端 clean build**（610 篇真实博客，M5 Pro）：

| `--jobs` | 端到端耗时（中位数） | 加速比 |
| --- | --- | --- |
| 1 | 305 ms | 1.00x |
| 4 | 207 ms | **1.47x** |
| 8 | 207 ms | 1.47x |
| 0（auto） | 209 ms | 1.46x |

**解析阶段隔离基准**（`BenchmarkParsePosts_Concurrency`）：

| 文章数 | jobs=1 | jobs=4 | 加速比 |
| --- | --- | --- | --- |
| 100 | 1.99 ms | 1.32 ms | 1.51x |
| 500 | 11.68 ms | 7.27 ms | 1.61x |

详细基准与画像见 `docs/superpowers/specs/2026-07-04-parallel-parse-design.md`。

### 6. 库 API 变化

新增两个公开 API：

```go
// ParsePostsWithOptionsConcurrent parses with an explicit worker count.
// 0 or negative means auto: min(NumCPU, 4). 1 means sequential.
func ParsePostsWithOptionsConcurrent(dir string, opts RenderOptions, concurrency int) ([]*Post, error)
func ParsePagesWithOptionsConcurrent(dir string, opts RenderOptions, concurrency int) ([]*Page, error)
```

旧 `ParsePostsWithOptions` / `ParsePagesWithOptions` 签名不变，内部默认走 auto 并发，行为对调用方透明。

---

## 文档

- `docs/guides/parallel-build.md` 已更新：v1.6 起 `--jobs` 同时作用于解析阶段；新增并发安全说明；新增 610 篇端到端基准数据。
- `docs/superpowers/specs/2026-07-04-parallel-parse-design.md` 是本次变更的设计文档（170 行），包含画像数据、并发安全分析、验收标准。

---

## Docker 镜像

Git tag 发布时会同时构建并推送 Docker Hub 镜像：

- `docker.io/mengbin92/gobin:v1.6.0`
- `docker.io/mengbin92/gobin:latest`

镜像支持：

- `linux/amd64`
- `linux/arm64`

运行示例：

```bash
docker run --rm -p 8080:8080 \
  -e GOBIN_AUTO_INIT=true \
  -v "$PWD:/site" \
  docker.io/mengbin92/gobin:v1.6.0
```

---

## 兼容性说明

- 本版本保持配置、模板、CLI 入口、增量构建、并行渲染、serve partial rebuild 完全向后兼容。
- `ParsePosts` / `ParsePages` / `ParsePostsWithOptions` / `ParsePagesWithOptions` 签名不变，默认 auto 并发，单文件退化为串行。
- `RenderOptions` 不新增字段，env hash 不变，增量构建 cache 兼容性保持。
- `--jobs` 语义扩展从"页面渲染 worker 数"为"解析与渲染 worker 数"；用户旧习惯（`--jobs 4`）直接受益，无需任何配置改动。
- 库 API：新增 `ParsePostsWithOptionsConcurrent` / `ParsePagesWithOptionsConcurrent`；`loadSiteBuildInputWithConcurrency` 取代裸 `loadSiteBuildInput` 在需要显式并发度的代码路径上使用；其余签名不变。

---

## 验证

发布前建议执行：

```bash
make test
go test -race ./internal/parser/... ./internal/generator/... ./cmd/gobin/commands/...
go vet ./...
gofmt -l $(find . -name '*.go' -not -path './website/*' -not -path './public/*')
make release-local
```
