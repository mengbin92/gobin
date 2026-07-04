# 图片优化管线使用指南

> Gobin v1.7.0 起随 `assets.images` 发布。本指南说明怎么启用管线、可调的参数、产物形状与排错思路。

## 1. 解决什么问题

真实博客（22 张图、7.1MB）启用 `assets.images` 后，hero image 体积典型值：

| 原图 | 480w variant | 800w variant | 1200w variant |
| --- | --- | --- | --- |
| `head/arduino.jpg` 649KB | **30KB (-95%)** | 68KB (-90%) | 151KB (-77%) |
| `head/server.jpg` 477KB | 30KB (-94%) | 73KB (-85%) | 167KB (-65%) |

每张图同时输出 2 套格式（`jpg` / `png`），由 `<picture><source>` 让浏览器按能力挑选。`<img>` 走 lazy / async 解码，移动端首屏 LCP 直接受益。

## 2. 快速上手

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

`enabled: false`（默认）时管线完全跳过，build 行为与 v1.6 字节级一致。开启后 `gobin build` 会：

1. 扫描所有 `![]()` 和 front matter `cover` / `image` / `thumbnail` / `hero` 字段
2. 对每个 distinct source 跑 `imaging.Transform` 生成多尺寸变体
3. 写 `publishDir/.gobin-images.json` manifest
4. postprocess 阶段把渲染出的 `<img>` 改写为 `<picture><source srcset>...<img srcset></picture>`

## 3. 模板：怎么引用图

### 3.1 Markdown `![]()` — 零配置

```markdown
![cover](/img/head/arduino.jpg)
```

渲染时 v1.7 自动把 `<img src="/img/head/arduino.jpg">` 改写为 `<picture>` 块（前提是该 path 已被 `ExtractImageRefs` 收集）。

### 3.2 Front matter `cover` — 零配置

```yaml
---
title: Post
cover: /img/head/arduino.jpg
---
```

需要模板里显式渲染 cover。默认 `single.html` 已加：

```html
{{- if .Post.Params.cover }}
{{ image .Post.Params.cover .Post.Title }}
{{- end }}
<h1>{{ .Post.Title }}</h1>
```

`{{ image src alt }}` 是 v1.7 新增的 helper，emit 原始 `<img>`，postprocess 改写为 `<picture>`。Helper 故意不预计算变体集合（template 在 image artifact 之前跑，不知道哪些 width/format 可用），让 postprocess 阶段做唯一的"改写真相"决策。

## 4. 产物形状

每个 source 路径产生 N×M 个变体（N = `srcset` 宽度、M = `formats`）：

```
public/img/head/arduino-480w.jpg   30KB
public/img/head/arduino-800w.jpg   68KB
public/img/head/arduino-1200w.jpg  151KB
public/img/head/arduino-480w.png  345KB
public/img/head/arduino-800w.png  768KB
public/img/head/arduino-1200w.png 1.6MB
public/.gobin-images.json
```

HTML 产物（多格式）：

```html
<picture>
  <source type="image/jpg" srcset="/img/head/arduino-480w.jpg 480w, /img/head/arduino-800w.jpg 800w, /img/head/arduino-1200w.jpg 1200w" sizes="(max-width: 800px) 100vw, 800px">
  <source type="image/png" srcset="/img/head/arduino-480w.png 480w, /img/head/arduino-800w.png 800w, /img/head/arduino-1200w.png 1200w" sizes="(max-width: 800px) 100vw, 800px">
  <img alt="cover" src="/img/head/arduino-1200w.jpg" srcset="..." sizes="..." loading="lazy" decoding="async">
</picture>
```

单格式时直接 emit `<img srcset>`，不包 `<picture>` 包装（少一层 DOM）。

## 5. 调参建议

| 场景 | 推荐配置 |
| --- | --- |
| 个人博客 | `srcset: [480, 800]`，`formats: [jpg]`，`quality: 75` |
| 设计/摄影站 | `srcset: [480, 800, 1200, 1920, 2560]`，`formats: [jpg, webp]`，`quality: 85` |
| 内网文档站 | `enabled: false`（v1.6 行为） |

`quality` 字段对应 JPEG 编码质量（1-100）；PNG 不受影响（PNG 是无损格式）。Q=75 是经验值，视觉无感且体积下降 ~30%；Q=85 适合对清晰度要求高的摄影站。

## 6. 局限性（v1.7）

- **WebP 编码**：stdlib 后端不支持。`formats: [webp]` 当前退化为 pass-through（不重编码，只复制原图到目标文件名）。等下次引入 `disintegration/imaging` 或 `bimg` 后会真正产出 WebP 变体。
- **AVIF**：同上，v1.7 不支持，列入 v1.8+ 候选。
- **增量构建**：当前每个 build 都重新生成所有变体（passthrough-write）；下个 patch 接入"源文件内容 hash 跳过"机制。
- **LQUIP / blurhash 占位图**：未做。
- **EXIF 保留**：当前重编码会丢失 EXIF（goldmark + stdlib jpeg 不保留元数据）；如需保留需在 v1.8 用 libvips 路径。

## 7. 排错

| 症状 | 可能原因 | 处理 |
| --- | --- | --- |
| `<picture>` 没出现在 HTML | cover / image 字段名拼错；或 `assets.images.enabled` 没设 | 改 front matter key 为 `cover` / `image` / `thumbnail` / `hero`；或 config 加 `enabled: true` |
| 变体没生成 | 路径引用是外链（`http://` / `//cdn`）；或路径包含 `..` 逃出 `staticDir` | 改用 `/img/...` 形式；或把外部资源下载到本地 |
| 启用后体积没下降 | 站点没有 `<img>` 引用（front matter 缺图、正文无图）；或图已经被 base template 调但 `image` helper 没渲染 | 用 `gobin check` + `--verbose` 看 `ImageStats.Sources` 是不是 0；用 `gobin check --assets` 验证产物存在 |
| `formats: [webp]` 启用后体积没变 | stdlib 后端 pass-through，WebP 没重编码 | 列入 v1.8+ WebP 后端；当前仅占位 |

## 8. 进阶：库 API

库用户可绕过 `assets.images` 配置直接调 `internal/imaging`：

```go
import "github.com/mengbin92/gobin/internal/imaging"

exec := imaging.NewStdlibExecutor()
variants, err := imaging.Transform(src, ".jpg", imaging.TransformOptions{
    Widths:  []int{480, 800, 1200},
    Formats: []string{"jpg", "png"},
    Quality: 80,
}, exec)
```

自定义 Executor：实现 `Decode` + `Encode` 接口，注入更高级的 filter（CatmullRom、Lanczos、WebP 编码器等）。Transform 不感知 executor 类型，扩展无侵入。
