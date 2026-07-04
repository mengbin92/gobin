# 图片优化管线 — 实现规格 (Spec)

> 日期：2026-07-04
> 状态：草稿，待 review
> 范围：v1.7.0
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
| 图片处理库 | `disintegration/imaging` | 纯 Go，无 cgo 依赖；Go 生态最稳；WebP 编码/解码齐全；AVIF 缺但可后续补 cgo 路径 |
| AVIF | 不做 v1 | disintegration 不支持；走 cgo + libvips 跨平台构建复杂；WebP 已经够；列入 v1.8 候选 |
| 尺寸策略 | `assets.images.srcset: [480, 800, 1200, 1920]` | 覆盖手机 / 平板 / 桌面 / 4K 主流断点；用户可自定义 |
| 默认 sizes | `(max-width: 800px) 100vw, 800px` | Jekyll 兼容且适配大部分博客主题 |
| 质量 | `quality: 80` | WebP q=80 视觉无感，体积下降 ~30% |
| 输出位置 | `publishDir/{原路径}-{width}w.{ext}` + `.{hash}.{ext}` | 兼容 v1.5 指纹（hash 后缀）；width 后缀在前方便按宽度调试 |
| HTML 改写 | v1.5 postprocess 扩展，正则识别 `<img src="...">` | 与 v1.5 改写 `<link>` / `<script>` 同链路，模式一致 |
| 缓存粒度 | 源文件内容 hash 跳过 | 与 v1.5 资产指纹同思路；增量构建 manifest 记录 |
| front matter 渲染 | 模板 helper `{{ image .Post.Cover }}` | 用户在模板里显式调用，避免入侵默认模板 |
| 错误传播 | 单图失败 → 警告 + 用原图，不中断构建 | 图片是装饰性元素，构建中断不划算 |

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

## 5. 架构：新增 `internal/imaging` 包

```
internal/imaging/
  imaging.go         # 公开 API：Transform(src, dstBase, opts) -> []*Output
  transform.go       # 多尺寸 + 多格式转换（disintegration 包装）
  format.go          # 格式判断 (jpg/png/webp)
  transform_test.go  # 单测
```

**公开 API**：

```go
type TransformOptions struct {
    Widths   []int
    Formats  []string  // "webp" / "jpg" / "png"
    Quality  int
}

type Output struct {
    Path   string  // 相对 publishDir 路径
    Width  int
    Format string
    Size   int64
}

func Transform(src []byte, srcExt string, opts TransformOptions) ([]Output, error)
```

## 6. 集成到生成器

### 6.1 解析期（`internal/parser`）

- Markdown `![alt](path)` 节点：解析器已处理（`goldmark` AST），无需改 parser
- 新增 helper：`ExtractImageRefs(posts []*parser.Post, pages []*parser.Page, cfg *config.Config) []ImageRef` —— 扫描所有 `![]()` 和 front matter `cover:`/`image:`/`thumbnail:`，去重后返回
- 关键：图片路径解析要跟随 Markdown 链接语义（相对路径基于 .md 文件目录）

### 6.2 生成期（`internal/generator`）

- 新增 `imagePipeline` artifact，与 `assets` / `postprocess` 同级
- 收集 `ImageRef` → 解析为绝对源路径 → 检查是否启用 → 调用 `imaging.Transform` → 写入 `publishDir`
- 输出 manifest：`.gobin-images.json`（参考 v1.5 `.gobin-assets.json`）
- 增量构建：根据源文件 hash + transform opts hash 跳过

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

## 7. 兼容性

- `assets.images.enabled: false`（默认）：v1.7 与 v1.6 字节级一致
- v1.5 资产指纹管线不破坏：未启用图片优化的图片仍走原 fingerprint 路径
- `RenderOptions` 不新增字段，env hash 不变
- 库 API：新增 `imaging.Transform` / `generator.ExtractImageRefs`；其余签名不变
- 现有 `assetFingerprinter` 不动（避免重复改 HTML 改写规则）

## 8. 测试方案

### 8.1 单元测试（`internal/imaging/transform_test.go`）

- `TestTransform_WebP`：jpg → 多尺寸 webp
- `TestTransform_PNG`：png → 多尺寸 webp（带 alpha）
- `TestTransform_Quality`：q=80 与 q=95 输出大小差异
- `TestTransform_EmptyFormats`：formats=[] 返回原尺寸单输出
- `TestTransform_InvalidFormat`：未知格式返回错误

### 8.2 集成测试（`internal/generator/generator_test.go`）

- `TestImagePipeline_DisabledIsByteIdentical`：关闭时与 v1.6 产物一致
- `TestImagePipeline_EnabledGeneratesPictureTags`：启用后 HTML 包含 `<picture>` + `<source>`
- `TestImagePipeline_FrontMatterCover`：front matter `cover:` 触发转换
- `TestImagePipeline_Incremental`：第二次构建无变化时跳过转换

### 8.3 端到端（手动）

- 真实 610 篇博客 22 张图：对比关闭 / 启用 两种产物的总大小与 HTML diff
- Lighthouse / PageSpeed 评分对比（可选，依赖环境）

## 9. 验收标准

- [ ] `assets.images.enabled: false` 产物与 v1.6 字节级一致
- [ ] 启用后，Markdown `![alt](path)` 和 front matter `cover:` 生成的 HTML 都含 `<picture>` 块
- [ ] WebP 转换正确，浏览器能加载
- [ ] 真实 22 张图启用后总大小下降 ≥ 30%
- [ ] 增量构建：未变化图片跳过重新转换
- [ ] 单图失败不阻断构建，输出 WARN 日志
- [ ] `go test -race ./...` 通过
- [ ] gofmt / go vet 通过
- [ ] 文档：`docs/guides/image-pipeline.md` + 本 spec + README 更新

## 10. 范围外（v1.8+ 候选）

- AVIF 格式（需 cgo + libvips）
- LQIP / blurhash 占位图
- 视频海报 / OG image 自动生成
- CDN 代理模式
- EXIF 保留
- 图片 CDN 缓存预热
