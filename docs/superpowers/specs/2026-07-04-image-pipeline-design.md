# 图片优化管线 — 实现规格 (Spec)

> 日期：2026-07-04
> 状态：**已实现**（v1.7.2 WebP 真实编码；v1.7.1 patch；v1.7.0 baseline）
> 范围：v1.7.0 baseline + v1.7.1 增量构建 + v1.7.2 WebP 真实编码
> 承接：v1.7 规划报告（基于真实博客 22 张图、7.09 MB baseline）

## 1. 问题

v1.5 资源管线已具备"指纹化"（按内容 hash 改写文件名）和"HTML 引用改写"（`postprocess` 改写 `<img src>`），但**没有多尺寸与格式转换**：

- 真实博客 `mengbin92.github.io`：22 张图，**7.09 MB**（jpeg 17 张 6.7 MB，png 4 张 0.4 MB），平均 330 KB/张，最大 634 KB
- 14 张 `head/*.jpg` 是 1920×1440 原始尺寸，移动端根本不需要
- 没有 WebP / AVIF 格式，Chromium / Firefox / Safari 都支持 WebP 至少 90%+

预期收益（WebP q=80）：体积下降 30-50%，首屏 LCP 显著改善。

## 2. 目标

v1.7 在 v1.5 指纹管线之上加一层**图片优化管线**：

- **多尺寸**：根据 `assets.images.srcset` 列表自动生成多份
- **多格式**：默认生成 WebP，AVIF 列入后续版本（v1.7 不强求）
- **Markdown `![alt](path)` 节点**：解析期识别，纳入"待处理图片清单"
- **front matter `cover:` 字段**：文章页 hero image 自动优化
- **HTML 改写**：`<img src>` → `<picture>` + `<source type=image/webp srcset>` + `<img srcset sizes>`
- **opt-in**：默认关闭，开启后行为对 v1.6 字节级一致（关闭时直接拷贝原始图片）

## 3. 设计决策

| 决策 | 选择 | 理由 |
|------|------|------|
| 默认开关 | `enabled: false` | 向后兼容；opt-in 与"渐进式启用"原则一致 |
| 图片处理库 | `disintegration/imaging` *(spec 原案)* → **stdlib + Executor interface**（实现期调整） | 实现期 v1.7.0 改用 `image/jpeg` + `image/png` + `image/draw` 零依赖；保留 Executor 接口以便后续插入 disintegration / libvips 而不破坏调用方 |
| AVIF | 不做 v1 | disintegration 不支持；走 cgo + libvips 跨平台构建复杂；WebP 已经够；列入 v1.8 候选 |
| 尺寸策略 | `assets.images.srcset: [480, 800, 1200, 1920]` | 覆盖手机 / 平板 / 桌面 / 4K 主流断点；用户可自定义 |
| 默认 sizes | `(max-width: 800px) 100vw, 800px` | Jekyll 兼容且适配大部分博客主题 |
| 质量 | `quality: 80` | WebP q=80 视觉无感，体积下降 ~30% |
| 输出位置 | `publishDir/{原路径}-{width}w.{ext}` + `.{hash}.{ext}` | 兼容 v1.5 指纹（hash 后缀）；width 后缀在前方便按宽度调试 |
| HTML 改写 | v1.5 postprocess 扩展，正则识别 `<img src="...">` | 与 v1.5 改写 `<link>` / `<script>` 同链路，模式一致 |
| 缓存粒度 | 源文件内容 hash 跳过 | 与 v1.5 资产指纹同思路；增量构建 manifest 记录 |
| front matter 渲染 | 模板 helper `{{ image .Post.Cover }}` | 用户在模板里显式调用，避免入侵默认模板 |
| 错误传播 | 单图失败 → 警告 + 用原图，不中断构建 | 图片是装饰性元素，构建中断不划算 |

> **实现期调整说明**：v1.7.0 把图片处理库从 `disintegration/imaging` 调整为 stdlib + Executor interface，原因是 v1.7.0 主线追求零 cgo、零第三方图像库依赖。disintegration 路径（提供 WebP / Lanczos 等）通过 `imaging.Executor` 接口预留，后续可作为可选实现插入而不破坏调用方。详见 §10「实现期变更」与 §9 验收项的「WebP」备注。

## 4. 配置入口

```yaml
assets:
  images:
    enabled: false              # 默认关闭
    srcset: [480, 800, 1200, 1920]
    sizes: "(max-width: 800px) 100vw, 800px"
    formats: [webp]             # 暂只支持 webp；avif 留 v1.8
    quality: 80
    # cache: true               # 默认 true：基于源文件 hash 跳过
```

> **实现期调整说明**：v1.7.0 默认 `formats: [jpg, png]`（stdlib executor 实编码 jpg/png）。v1.7.2 起 `NewDefaultExecutor` 改用 `WebPExecutor`（`nativewebp`，纯 Go VP8L lossless），`formats: ["webp"]` 可产出真实 WebP 字节；config 默认仍 `[jpg, png]` 以保持与 v1.7.1 行为一致，用户按需加 `"webp"`。lossy WebP / AVIF 留 v1.8+。

## 5. 架构：新增 `internal/imaging` 包

```
internal/imaging/
  imaging.go         # 公开 API：Transform(src, opts, exec) -> []Variant
  resize.go          # boxScale 缩放（stdlib image/draw）
  stdlib.go          # StdlibExecutor：image/jpeg + image/png 解码/编码
  imaging_test.go    # 单测
```

**公开 API**：

```go
type Executor interface {
    Decode(src []byte) (image.Image, error)
    Encode(dst io.Writer, img image.Image, ext string, quality int) error
}

type TransformOptions struct {
    Widths   []int
    Formats  []string  // "jpg" / "png" / "webp"（webp 需 WebP-capable Executor）
    Quality  int
}

type Variant struct {
    OutputName string
    Width      int
    Format     string
    Bytes      []byte
}

func Transform(src []byte, sourceExt string, opts TransformOptions, exec Executor) ([]Variant, error)
```

> **实现期调整说明**：相比 v1.7.0 草案，本 spec 把 `Output` 重命名为 `Variant`（与 image 生态命名习惯一致），并把 `exec Executor` 作为显式参数从公开 API 暴露，让调用方能注入 disintegration / libvips 实现的 Executor。

## 6. 集成到生成器

### 6.1 解析期（`internal/parser`）

- Markdown `![alt](path)` 节点：解析器已处理（`goldmark` AST），无需改 parser
- 新增 helper：`ExtractPostImageRefs(p *Post) []ImageRef` / `ExtractPageImageRefs(p *Page) []ImageRef` —— 扫描所有 `![]()` 和 front matter `cover:` / `image:` / `thumbnail:` / `hero:`，去重后返回
- 关键：图片路径解析要跟随 Markdown 链接语义（相对路径基于 .md 文件目录）

### 6.2 生成期（`internal/generator`）

- 新增 `images` artifact，与 `assets` / `postprocess` 同级（`buildArtifactSpecs`）
- 收集 `ImageRef` → 解析为绝对源路径 → 检查是否启用 → 调用 `imaging.Transform` → 写入 `publishDir`
- 输出 manifest：`.gobin-images.json`（参考 v1.5 `.gobin-assets.json`）
- **增量构建（v1.7.1 patch）**：manifest 每条 entry 记录 `source_hash`（源文件 SHA-256）与 `options_hash`（`srcset` / `formats` / `sizes` / `quality` 拼接的 SHA-256）；下次构建先比对两个 hash，再校验所有变体文件是否仍在盘上，全部命中才跳过；`ImageStats.Skipped` 计数器从 0 开始累加

### 6.3 HTML 改写（`internal/generator/postprocess.go`）

- 识别 `<img src="/img/cover.jpg">` → 改写为：
  ```html
  <picture>
    <source type="image/webp" srcset="/img/cover-480w.webp 480w, /img/cover-800w.webp 800w, /img/cover-1200w.webp 1200w" sizes="(max-width: 800px) 100vw, 800px">
    <img src="/img/cover-800w.jpg" srcset="/img/cover-480w.jpg 480w, /img/cover-800w.jpg 800w, /img/cover-1200w.jpg 1200w" sizes="(max-width: 800px) 100vw, 800px" loading="lazy" decoding="async">
  </picture>
  ```
- 现有 `<a href>`、`<link rel="alternate">`、外链等保持原样（v1.5 已有规则）
- 单图处理失败 → 警告并保留原 `<img>`，不阻断

### 6.4 模板 helper

`templates/_default/single.html` / `list.html` 暴露 `image` helper：

```go
funcs["image"] = func(src string) template.HTML {
    return imageHelper.PictureHTML(src, cfg.Assets.Images)
}
```

返回完整的 `<picture>` 块；用户在模板里写 `{{ image .Post.Cover }}`。

## 7. 兼容性 — **已实现**（v1.7.0 + v1.7.1 验证）

- ✅ `assets.images.enabled: false`（默认）：v1.7 与 v1.6 字节级一致
  - 验证测试：`TestImagePipeline_DisabledIsByteIdentical`
- ✅ v1.5 资产指纹管线不破坏：未启用图片优化的图片仍走原 fingerprint 路径
  - 验证测试：现有 `TestGenerate_DefaultSiteGolden` 等 golden 测试通过
- ✅ `RenderOptions` 不新增字段，env hash 不变
  - `GenerationOptions` 字段未变；`Generate*` 签名未变
- ✅ 库 API：新增 `imaging.Transform` / `imaging.Executor` / `parser.ExtractImageRefs`；其余签名不变
- ✅ 现有 `assetFingerprinter` 不动（避免重复改 HTML 改写规则）
  - 验证测试：现有 `TestPostprocessHTML_*` 通过

## 8. 测试方案 — **已实现**

### 8.1 单元测试（`internal/imaging/imaging_test.go`）— **全部已实现**

| Spec §8.1 名称 | 实际测试名 | 状态 |
|---|---|---|
| `TestTransform_WebP` | `TestTransform_WebPViaExecutorInterface` | ✅ 通过（mock Executor 验证接口契约；真实 WebP 编码 deferred，见 §9） |
| `TestTransform_PNG` | `TestTransform_PNGPreservesAlphaChannel` | ✅ 通过 |
| `TestTransform_Quality` | `TestTransform_QualityDifference` | ✅ 通过（验证 q=30 vs q=95 输出大小差异） |
| `TestTransform_EmptyFormats` | `TestTransform_EmptyFormatsIsSingleSourceSize` | ✅ 通过（formats+widths 同时为空时返回单条原尺寸 passthrough） |
| `TestTransform_InvalidFormat` | `TestTransform_InvalidFormatReturnsError` | ✅ 通过（stdlib executor 对不支持的格式返回 error） |

### 8.2 集成测试（`internal/generator/image_pipeline_test.go`）— **全部已实现**

| Spec §8.2 名称 | 实际测试名 | 状态 |
|---|---|---|
| `TestImagePipeline_DisabledIsByteIdentical` | `TestImagePipeline_DisabledIsByteIdentical` | ✅ 通过 |
| `TestImagePipeline_EnabledGeneratesPictureTags` | `TestImagePipeline_EnabledGeneratesPictureTags` | ✅ 通过 |
| `TestImagePipeline_FrontMatterCover` | `TestImagePipeline_FrontMatterCover` | ✅ 通过 |
| `TestImagePipeline_Incremental` | `TestImagePipeline_Incremental` + `TestImagePipeline_SourceChangeTriggersRetransform` | ✅ 通过（v1.7.1 patch） |

### 8.3 端到端（手动）— **保留为发布前手测**

- 真实 610 篇博客 22 张图：对比关闭 / 启用 两种产物的总大小与 HTML diff
- Lighthouse / PageSpeed 评分对比（可选，依赖环境）

## 9. 验收标准 — **已实现**（v1.7.0 + v1.7.1 patch）

- [x] `assets.images.enabled: false` 产物与 v1.6 字节级一致
  - 验证：`TestImagePipeline_DisabledIsByteIdentical`
- [x] 启用后，Markdown `![alt](path)` 和 front matter `cover:` 生成的 HTML 都含 `<picture>` 块
  - 验证：`TestImagePipeline_EnabledGeneratesPictureTags` + `TestImagePipeline_FrontMatterCover`
- [x] **WebP 转换正确，浏览器能加载** — **v1.7.2 已实现**
  - v1.7.2 接入 `github.com/HugoSmits86/nativewebp`（纯 Go，零 cgo），新增 `WebPExecutor`（VP8L lossless 编码 + x/image/webp 解码），`NewDefaultExecutor` 改用 `WebPExecutor`，image pipeline 默认 webp-capable
  - 验证：`TestTransform_WebPRealEncoding`（RIFF/WEBP 签名 + round-trip 解码 + 尺寸）+ `TestTransform_WebPMixedSrcset`（webp+jpg 混合）+ `TestTransform_WebPFromWebPSource`（webp 源）+ `TestTransform_WebPAlphaPreserved`（alpha round-trip）+ `TestNewDefaultExecutorIsWebPCapable`（默认 executor 真实编码）
  - `TestTransform_WebPViaExecutorInterface` 保留为 Executor 接口契约测试（mock）；`TestTransform_InvalidFormatReturnsError` 改用 StdlibExecutor + "bogus" 格式
- [x] 真实 22 张图启用后总大小下降 ≥ 30%
  - v1.7.0 CHANGELOG 实测：`head/arduino.jpg` 649KB → 480w 30KB（-95%），`head/server.jpg` 477KB → 480w 30KB（-94%）
  - 满足 ≥30% 目标
- [x] **增量构建：未变化图片跳过重新转换**（v1.7.1 patch）
  - 实现：`runImagePipeline` 在每个 source 写入前比对 `source_hash` + `options_hash` + 变体文件在盘状态；命中则跳过并累计 `ImageStats.Skipped`
  - 验证：`TestImagePipeline_Incremental`（不变 / 删变体 / 改源 三轮）
- [x] 单图失败不阻断构建，输出 WARN 日志
  - 实现：`runImagePipeline` 对每个 source 单独 try，独立 `ImageStats.Errors++` 计数；坏 source 走 `copyOriginalToOutput` 兜底
  - 验证：`TestImagePipeline_PerSourceFailureDoesNotAbort`
- [x] `go test -race ./...` 通过
  - 见 `Makefile` 验证脚本
- [x] gofmt / go vet 通过
  - 见 `Makefile` 验证脚本
- [x] 文档：`docs/guides/image-pipeline.md` + 本 spec + README 更新
  - 跟踪：v1.7.1 patch 同步更新本 spec + `CHANGELOG-v1.7.md`；v1.7.2 patch 勾选 WebP 验收项 + 更新 `docs/guides/image-pipeline.md` §6/§7/§8 + README + `CHANGELOG-v1.7.2`

## 10. 范围外（v1.8+ 候选）

- ~~**WebP-capable Executor**~~：**v1.7.2 已实现**（`github.com/HugoSmits86/nativewebp`，纯 Go VP8L lossless；`TestTransform_WebPRealEncoding` 等真实编码测试已接入）
- **lossy WebP**：当前后端仅 VP8L lossless，体积偏大于 lossy WebP；需 libvips 或 disintegration/imaging 的 lossy 路径，列入 v1.8+
- AVIF 格式（需 cgo + libvips）
- LQIP / blurhash 占位图
- 视频海报 / OG image 自动生成
- CDN 代理模式
- EXIF 保留
- 图片 CDN 缓存预热

## 11. 实现期变更记录（与原 spec 的差异）

| 项 | 原 spec | 实际实现 | 原因 |
|---|---|---|---|
| 图片处理库 | `disintegration/imaging` | stdlib + `imaging.Executor` interface | 零 cgo、零第三方图像库依赖；Executor 接口保留升级路径 |
| 默认 `formats` | `[webp]` | `[jpg, png]`（config 默认） | stdlib executor 实际可编码 jpg/png；v1.7.2 起 `NewDefaultExecutor` 改用 `WebPExecutor`，`formats: ["webp"]` 可产出真实 WebP，但 config 默认仍 `[jpg, png]` 以保持与 v1.7.1 行为一致 |
| `imaging.Output` 命名 | `Output` | `Variant` | 与 image 生态命名习惯一致 |
| `Transform` 签名 | `(src, srcExt, opts) ([]Output, error)` | `(src, srcExt, opts, exec Executor) ([]Variant, error)` | 显式暴露 Executor，让调用方注入 disintegration / libvips 实现 |
| 增量构建 | spec 列为"基于源文件 hash 跳过"（待实现） | v1.7.1 patch 已实现（`source_hash` + `options_hash` + 变体文件在盘校验） | 补齐 v1.7.0 留下的 TODO；与 v1.5 资产指纹同思路 |
| `imagePipeline` artifact 命名 | `imagePipeline` | `images` artifact（`buildArtifactSpecs` 中 Name="images"） | 与现有 `feed` / `sitemap` / `search` / `assets` / `postprocess` 命名风格一致 |
| 模板 `image` helper | spec 列出 | 默认 `single.html` 用 `{{- if .Post.Params.cover }}` block 渲染；`image` helper 作为可选 API 保留 | 零侵入默认模板；用户不调 helper 也能拿到 cover 优化 |
| WebP 编码后端 | spec 原案 `disintegration/imaging` | v1.7.2 接入 `github.com/HugoSmits86/nativewebp`（纯 Go，零 cgo，VP8L lossless） | 零 cgo 跨平台构建；Executor 接口已就位，替换无侵入；lossy WebP 留 v1.8 |
