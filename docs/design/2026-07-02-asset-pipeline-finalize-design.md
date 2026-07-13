# 静态资源管线收口 — 实现规格 (Spec)

> 日期：2026-07-02
> 状态：草稿，待 review
> 范围：v1.5.0
> 承接：`README.md` 第四阶段 `[ ] 指纹资源和更完整的资源管线`

## 1. 目标

把 v1.4.0 已经搭起来但**未串通**的指纹能力在 v1.5.0 收口：

1. HTML 模板里**实际**输出的 `<link href>` / `<script src>` / `<img src>` 与磁盘上**真实**的资源文件名一致——目前只有 `assetURL` 模板函数会重写，写死的字符串资源（如用户自定义模板里的 `href="/css/site.css"`）指纹化后会出现 404。
2. `filename` 指纹模式下，**内容 hash 一致性可验证**：fingerprint 写盘前用同一份 hash 计算，发布前可用脚本验证磁盘上的 `name.<hash>.ext` 文件中 `hash` 与文件内容 hash 一致。
3. 按 MIME 大类（CSS / JS / img）做最小化分类：决定哪些扩展名走 `filename` 指纹、哪些走 `query`、哪些不处理。
4. （来自 P2.1 尾巴）`scripts/check-benchmark.sh` 升级为**相对上次基线的回归门禁**，替代 order-of-magnitude 阈值。

## 2. 设计决策

| 决策 | 选择 | 理由 |
|------|------|------|
| HTML 引用改写时机 | 构建期后处理（与 `minify` 同一阶段），在所有 page spec 写盘后对 `publishDir` 扫一遍 | 与 minify 一样属于"产物最终化"；不在渲染期改写可避免与并行的 race |
| 改写范围 | 只动 `href` / `src` 属性；不动 CSS 内的 `url(...)`、JS 内的字符串 | CSS / JS 改写是另一个 spec 的事；超出 v1.5.0 范围 |
| MIME 分类位置 | `internal/generator/assets.go` 内新增 `categorizeAsset(ext string) assetCategory` | 与现有 asset 收集入口同文件，低侵入 |
| 哪类资源走 filename 指纹 | `css` / `js` / `image` 三类按现有 `Extensions` 配置；其它（`font` / `video` / `doc`）默认不参与 | 保守扩列，3 类已能解决绝大部分 cache-bust 场景 |
| Hash 验证工具 | `gobin check --assets` 子模式 | 复用 `gobin check` 已有 dry-run 骨架，零新命令 |
| 回归门禁的数据源 | `git show HEAD:benchmark-results.txt` 拿到上一次的基线 | CI 不需要新 artifact；本地跑也行 |

## 3. 目录结构

```
internal/generator/
  assets.go              # 新增 categorizeAsset、collectAssetsByCategory
  postprocess.go         # 新增（post-render HTML 引用改写 + asset hash 验证）
  postprocess_test.go
cmd/gobin/commands/
  check.go               # 扩展 --assets 子模式
  check_test.go
scripts/
  check-benchmark.sh     # 升级为相对回归门禁
docs/guides/
  asset-pipeline.md      # 新增：用户视角的资源管线说明
```

## 4. 行为契约

### 4.1 HTML 引用改写

- 输入：写盘后 `publishDir` 下的所有 `.html`
- 处理：解析每个 `<link href>`、`<script src>`、`<img src>`，若目标路径在 `manifest` 内且策略是 `filename`，改写为 `name.<hash>.ext`
- 不改写：`<a href>`（避免误改站外 / 文章内跳转）、`<link rel="alternate">`（RSS/Atom 引用，不需 cache-bust）、`<form action>`（POST URL）
- 不改写：磁盘上找不到的链接（404 行为留给模板作者）
- 不改写：跨站 URL（`http://` / `https://` / `//`）

### 4.2 `gobin check --assets`

- 不写盘；只读
- 对每个 manifest 中的 `filename` 指纹资源：
  - 读磁盘上 `publishDir/<output>` 内容 → 算 hash
  - 与 manifest 中的 hash 比对
  - 不一致：报告 `<output>: hash mismatch (expected X, got Y)`
- 退出码：0 全部一致，1 有 mismatch

### 4.3 MIME 分类

```go
type assetCategory string
const (
    catCSS   assetCategory = "css"
    catJS    assetCategory = "js"
    catImage assetCategory = "image"
    catOther assetCategory = "other"
)

func categorizeAsset(ext string) assetCategory {
    switch strings.ToLower(ext) {
    case ".css": return catCSS
    case ".js", ".mjs": return catJS
    case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".avif", ".svg": return catImage
    default: return catOther
    }
}
```

`AssetsFingerprintConfig.Extensions` 默认列表按 MIME 分类自动生成（不再写死扩展名数组）：

```go
func DefaultAssetsFingerprintExtensions() []string {
    return []string{".css", ".js", ".mjs", ".png", ".jpg", ".jpeg", ".gif", ".webp", ".avif", ".svg"}
}
```

### 4.4 相对回归门禁

- 旧 `check-benchmark.sh` 阈值（10/1/20/20/50 ms）保留为**绝对上限**——不能比"100 倍慢"更慢
- 新增**相对回归门禁**：当前 ns/op > 上次 `HEAD` 基线 × 1.5（红）/ × 1.2（黄）
- 红 fail、黄 warn（CI 默认 fail-on-warn；本地默认 warn）
- 例外：标准差 > 30% 时跳过相对判断（基准噪声）

## 5. 不在范围

- CSS 内 `url(...)` 改写
- JS 内 `import` / 字符串引用改写
- 图片优化（独立的 v1.5+ 候选）
- 把 `gobin check` 升级为独立子命令群（保持现有 `--assets` 子模式）

## 6. 风险

- HTML 引用改写是后处理，与 `--incremental` 的产物指纹兼容性需要验证：改写后的 HTML 字节变化，但只要 manifest 的 `build_env_hash` 不变，env 失配检测会触发全量重建——这是 by design
- 相对回归门禁依赖 `git show HEAD:benchmark-results.txt`，如果上一次基线是手写 / 错的，会误报。CI 在合入前必须跑一次基线
- 1.5x 阈值是启发式，可后续调整

## 7. 验证

```bash
go test ./... -race
make lint
make benchmark BENCH_TIME=2s
# 新建 example-site，跑 build 后用 gobin check --assets 验证 hash 一致
gobin build --minify
gobin check --assets  # 期望: 空输出, 退出码 0
# 编辑 _posts 中一篇
gobin build --incremental --clean=false
# 改一个 .css 文件内容（不改 filename）
gobin build --clean=false
gobin check --assets  # 期望: 报告 hash 不一致
```
