# 并行构建使用指南

> Gobin v1.4.0 起随 `--jobs` 发布（v1.6.0 起 `--jobs` 同时作用于解析阶段）。本指南说明 `--jobs` 的取值含义、产物一致性边界，以及怎么挑一个合适的工作并发数。

## 1. 解决什么问题

默认情况下，页面渲染阶段是单线程顺序写盘的。对于 500+ 篇 post 的站点，这一阶段在多核机器上只占一个核的算力，其余核心空转。`--jobs N` 让 Gobin 用 N 个 worker **并发** 渲染所有 `PageSpec`（文章页、列表页、taxonomy 页、404），把多核利用起来。

## 2. 快速上手

```bash
# 默认（自动）：min(NumCPU, 4)
gobin build

# 显式指定 4 个 worker
gobin build --jobs 4

# 强制串行（关掉并行）
gobin build --jobs 1
```

`--jobs` 的取值语义：

| 取值 | 行为 |
|------|------|
| `0`（默认） | 自动 = `min(runtime.NumCPU(), 4)` |
| `1` | 强制串行 |
| `>1` | 显式 N 个 worker，**不封顶**（用户自负责任） |

## 3. 覆盖范围

并行作用于**两个阶段**（v1.4 起页面渲染，v1.6 起解析）：

1. **Markdown 解析**（v1.6+）—— `parser.ParsePostsWithOptions` / `ParsePagesWithOptions` 把 WalkDir 的文件列表按 stripe 分给 N 个 worker 并行解析。Parse 函数是纯函数、shortcode registry 是只读 map，所以并行无风险。
2. **页面渲染**（v1.4+）—— `PageSpec` 列表按 stripe 分给 N 个 worker 并发渲染并 `os.WriteFile`。

每个阶段各自 stripe 分配，worker 内 local 统计在末尾合并，首个错误经原子标志位让其它 worker 尽快停下。两个阶段共用同一 `--jobs` 值，不需要分别设置。

**不在并行范围**内（保持串行）：

- feed / atom / sitemap / search index / aliases / robots 等聚合产物（v1.5 实测仅占总耗时 ~2%，不值得做）；
- 静态资源复制；
- `cleanOutput` 的目录清理；
- 写 `buildManifest` / 写 assets manifest。

原因是这些阶段要么是单文件单写者（manifest）、要么强依赖全局顺序（聚合 / 资产 manifest），把它们一起并行收益小、风险大。

## 4. 产物一致性

- 并行与串行产物**字节级一致**：已用 `diff -r` 对同一站点 `gobin build --jobs 1` 与 `gobin build` 输出做过验证。
- 与 `--incremental` 正交：编辑 1 篇 post → 只重渲染 1 页，与是否并行无关。
- 与 `--minify` 正交：minify 在每个 page spec 写盘前串行做，路径不动。

## 5. 怎么挑 N

经验值（仓库基准参考）：

| 站点规模 | 推荐 N | 备注 |
|----------|--------|------|
| < 100 篇 | `0`（默认）或 `1` | 解析 + 渲染都 < 100 ms，并行收益不明显 |
| 100–1000 篇 | `0`（默认）| 4 worker 在多核机器上拿 1.3–1.5x 端到端收益；自动封顶避坑 |
| 1000+ 篇 | `4–8` | 把 N 显式调大；如果模板很重可以上 16 |
| 模板极重（大量 `range` / 递归） | `min(NumCPU, 16)` | 偏 CPU 密集，worker 数拉满收益更明显 |
| 网络盘 / NFS / 慢 SSD | `1` 或 `2` | I/O 竞争比并行收益大 |

**反模式**：

- 设 `N = NumCPU` 在 I/O 密集场景下可能比串行更慢（仓库基准：10 核机器 NumCPU=10 → 124 ms 慢于串行 106 ms）。这就是默认封顶 4 的原因。
- 设 `N = 0` 之外的大数但磁盘是单盘 HDD：会出现大量小文件争抢磁头，反而劣化。

## 6. 怎么验证真的快了

`make benchmark` 提供了一个并行对照基准 `BenchmarkBuildFull_Concurrency`：

```
BenchmarkBuildFull_Concurrency/posts=500/jobs=1   ~105 ms
BenchmarkBuildFull_Concurrency/posts=500/jobs=2   ~94 ms
BenchmarkBuildFull_Concurrency/posts=500/jobs=4   ~89 ms   ← 自动（默认）
BenchmarkBuildFull_Concurrency/posts=500/jobs=0   ~89 ms
```

对照方法：

1. 在你的目标硬件上跑一次：`make benchmark`。
2. 对比 `jobs=1` 与 `jobs=4`（或你设的 N）的 ns/op。
3. 如果差距 < 5%，说明你的瓶颈不在渲染阶段——检查是否走到了磁盘 I/O 慢盘、是否启用了 `--minify`、是否走了 `--incremental` 的退化路径。

**v1.6 起** `--jobs` 同时作用于解析阶段。同一 610 篇真实博客端到端（clean build）：**305 ms → 207 ms（约 1.47x）**。HTML/资产产物字节级一致；仅 feed/sitemap 时间戳差异。详见 `docs/design/2026-07-04-parallel-parse-design.md`。

## 7. 与 race detector

`go test -race ./...` 已经覆盖了并行路径：

- 并行渲染：`internal/generator/parallel_test.go` + `incremental_bench_test.go`
- 并行解析：v1.6 起新增 `internal/parser/parallel_test.go`（含 8 个测试覆盖等价性、错误传播、排序稳定）。如果你的 fork 改了渲染逻辑导致并发问题，最先怀疑的点：

- 共享的 hash 缓存。v1.4 已经把 `assetFingerprinter` 内部 hash 缓存加锁（commit `4afe359`），后续如新增 worker 间共享 map 必须加锁或用 sync.Map。
- 全局 `template.Template` 复用。Go 的 `html/template` 自身是并发安全的，但如果你 fork 后加了模板级缓存，要注意 cache key 竞争。
- 随机数源。如需在模板里 `rand`，用 `math/rand/v2` 的全局 `func` 而不是构造 `*rand.Rand` 共享。

## 8. 排错

| 症状 | 可能原因 | 处理 |
|------|----------|------|
| `--jobs 8` 比 `--jobs 1` 慢 | I/O 竞争（慢盘 / 大量小文件） | 降到 `4` 或 `2`；或换 SSD |
| 并行构建间歇性 `os.WriteFile: too many open files` | worker 太多、fd 没及时关 | 降低 N；或在 `ulimit -n` 充足的环境跑 |
| 产物文件偶尔缺一两个 | 早期未捕获的并发 bug | 跑 `gobin build --jobs 1` 复现；如不复现，附带 `go test -race ./...` 输出提 issue |
| `go test -race` 命中 data race | 你的 fork 改了共享状态 | 检查新增的 worker 间共享 map / 缓存 |
| CI 上 benchmark 时快时慢 | CI runner 不可控 | 只看相对值（`jobs=N / jobs=1`），不要跨 runner 比绝对值 |

## 9. 与其它标志的搭配

```bash
# 增量 + 并行（最常见）
gobin build --incremental --clean=false --jobs 4

# 串行 + 保守压缩（适合调试）
gobin build --jobs 1 --minify
```

> `--jobs` 与 `--incremental` 同时启用时，--incremental 先决定哪些页要渲染，再把这些页分配到 N 个 worker。所以**编辑 1 篇 post → 1 worker 干 1 个活**，与 jobs 值无关。
