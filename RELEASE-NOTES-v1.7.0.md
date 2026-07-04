# Gobin v1.7.0 发布说明

## 发布日期 - 2026-07-04

Gobin v1.7.0 是一次面向"图片优化"的功能版本。本次发布在 v1.6.0 的基础上加入 `assets.images` 管线：自动从 Markdown `![]()` 与 front matter `cover` / `image` / `thumbnail` / `hero` 字段收集图引用，生成多尺寸 + 多格式变体，postprocess 阶段把 `<img>` 改写为 `<picture><source srcset>`。真实博客 22 张图、arduino.jpg 649KB 启用后变体 480w 仅 30KB（-95%），对移动端首屏 LCP 直接受益。

---

## 亮点

- **`assets.images` 管线**（opt-in）：Markdown 与 front matter 自动参与；产物改 `<picture>` + `srcset`；公开 Executor interface 让未来 WebP/AVIF 接入零侵入。
- **零侵入 front matter `cover`**：default `single.html` 已加 `{{- if .Post.Params.cover }}` block，golden 测试保证无 cover post 输出与 v1.6 字节级一致。
- **完全向后兼容**：`assets.images.enabled: false`（默认）时构建与 v1.6 字节级一致；其余 API 零变动。
- **stdlib 后端零依赖**：`internal/imaging` 用 `image/jpeg` + `image/png` + `image/draw`（手写 box filter）实现，不需要 cgo 也不需要新 module。

---

## 升级方式

```bash
go install github.com/mengbin92/gobin/cmd/gobin@v1.7.0
```

或从 GitHub Releases 下载对应平台的压缩包，并使用 `SHA256SUMS` 校验。

Docker 用户可以使用：

```bash
docker pull docker.io/mengbin92/gobin:v1.7.0
```

---

## 图片优化管线

### 1. 快速启用

```yaml
# config.yaml
assets:
  images:
    enabled: true
    srcset: [480, 800, 1200, 1920]
    sizes: "(max-width: 800px) 100vw, 800px"
    formats: [jpg, png]
    quality: 75
```

跑 `gobin build` 即可。无需改任何模板——v1.7 默认 `single.html` 已加 cover image 渲染，所有 Markdown `![]()` 也自动被 postprocess 改写。

### 2. 体积数据（真实 22 张图、7.1MB → 大幅下降）

| 原图 | 480w | 800w | 1200w |
| --- | --- | --- | --- |
| `head/arduino.jpg` 649KB | **30KB (-95%)** | 68KB (-90%) | 151KB (-77%) |
| `head/server.jpg` 477KB | 30KB (-94%) | 73KB (-85%) | 167KB (-65%) |

### 3. 产物形状

每个 source 路径生成 N×M 个变体（N = `srcset` 宽度、M = `formats`）：

```
public/img/head/arduino-480w.jpg   30KB
public/img/head/arduino-800w.jpg   68KB
public/img/head/arduino-1200w.jpg  151KB
public/img/head/arduino-480w.png  345KB
public/img/head/arduino-800w.png  768KB
public/img/head/arduino-1200w.png 1.6MB
public/.gobin-images.json
```

HTML 产物（多格式路径）：

```html
<picture>
  <source type="image/jpg" srcset="/img/head/arduino-480w.jpg 480w, /img/head/arduino-800w.jpg 800w, /img/head/arduino-1200w.jpg 1200w" sizes="(max-width: 800px) 100vw, 800px">
  <source type="image/png" srcset="/img/head/arduino-480w.png 480w, /img/head/arduino-800w.png 800w, /img/head/arduino-1200w.png 1200w" sizes="(max-width: 800px) 100vw, 800px">
  <img alt="cover" src="/img/head/arduino-1200w.jpg" srcset="..." sizes="..." loading="lazy" decoding="async">
</picture>
```

单格式时直接 emit `<img srcset>`，不包 `<picture>` 包装。

### 4. 配置项

| 字段 | 默认 | 说明 |
| --- | --- | --- |
| `enabled` | `false` | 开关 |
| `srcset` | `[480, 800, 1200, 1920]` | 宽度列表；超过 source 宽度的会被丢弃（避免 upscale） |
| `sizes` | `(max-width: 800px) 100vw, 800px` | `sizes` 属性，浏览器用它挑变体 |
| `formats` | `[jpg, png]` | 输出格式；当前 stdlib 后端 WebP/AVIF pass-through |
| `quality` | `75` | JPEG 质量（1-100）；PNG 不受影响（无损） |

### 5. 模板：怎么引用图

**Markdown**（零配置）：

```markdown
![cover](/img/head/arduino.jpg)
```

**Front matter cover**（零配置，default `single.html` 已加）：

```yaml
---
title: Post
cover: /img/head/arduino.jpg
---
```

**自定义 helper**（v1.7 新增）：

```html
{{ image "/img/head/arduino.jpg" "alt text" }}
```

emit 原始 `<img>`，postprocess 改写为 `<picture>`。

### 6. 局限性（v1.7）

- **WebP 编码**：stdlib 后端不支持。`formats: [webp]` 当前退化为 pass-through（不重编码）。v1.8+ 接入 `disintegration/imaging` 或 `libvips` 后真正产出 WebP 变体。
- **AVIF**：同上，列入 v1.8+ 候选。
- **增量构建**：当前每次 build 重新生成所有变体；下个 patch 接入"源文件内容 hash 跳过"机制。
- **LQIP / blurhash 占位图**：未做。
- **EXIF 保留**：当前重编码丢失 EXIF；如需保留需走 libvips 路径。

---

## 库 API 变化

新增：

```go
// imaging
type Executor interface { Decode(src []byte) (image.Image, error); Encode(dst io.Writer, img image.Image, ext string, quality int) error }
type TransformOptions struct { Widths []int; Formats []string; Quality int }
type Variant struct { OutputName string; Width int; Format string; Bytes []byte }
func Transform(src []byte, sourceExt string, opts TransformOptions, exec Executor) ([]Variant, error)
func NewStdlibExecutor() *StdlibExecutor

// parser
type ImageRef struct { Ref string; Source string; Kind string }
func ExtractPostImageRefs(p *Post) []ImageRef
func ExtractPageImageRefs(p *Page) []ImageRef
```

旧 `Generate` / `GenerateWithPages` / `GenerateWithPagesResult` / `GenerateWithOptions` 签名不变，行为对调用方透明。

---

## 兼容性说明

- 本版本保持配置、模板、CLI 入口、增量构建、并行构建、shortcodes、serve partial rebuild 完全向后兼容。
- `assets.images.enabled: false`（默认）：v1.7 与 v1.6 字节级一致。
- 模板默认 `single.html` 加了 `{{- if .Post.Params.cover }}` block；用 `{{- -}}` trim 标记保证无 cover 时输出与 v1.6 字节级一致（golden 测试覆盖）。
- 增量构建未参与 image pipeline（每次 build 重新生成所有变体），是已知未优化点。
- 库 API：新增 `imaging.*` / `ExtractPostImageRefs` / `ExtractPageImageRefs` / `ImageRef`；其余签名不变。

---

## 验证

发布前建议执行：

```bash
make test
go test -race ./internal/parser/... ./internal/generator/... ./cmd/gobin/commands/...
go vet ./...
gofmt -l $(find . -name '*.go' -not -path './website/*' -not -path './public/*' -not -path './example-site/*')
make release-local
```
