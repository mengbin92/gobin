# Gobin v1.7.0 更新日志

## 发布日期 - 2026-07-04

Gobin v1.7.0 是一次面向"图片优化"的功能版本。性能画像（基于真实博客 610 篇 + pprof CPU profile）确认 v1.6 解析并行化之后，剩下可优化的就是产物侧的图。v1.7.0 把 Markdown `![]()` 与 front matter `cover` / `image` / `thumbnail` / `hero` 字段自动接入多尺寸 + 多格式变体生成，postprocess 阶段把渲染出的 `<img>` 改写为 `<picture><source srcset>...<img srcset></picture>`。真实博客 22 张图、arduino.jpg 649KB 启用后变体 480w 30KB（-95%）。本次发布保持配置、模板、CLI 入口、并行构建、增量构建、shortcodes、serve partial rebuild 既有行为完全向后兼容。

---

## 新增功能

### 图片优化管线（v1.7）

- 新增 `assets.images` 配置段（`enabled` / `srcset` / `sizes` / `formats` / `quality`），opt-in（默认 `enabled: false`）。关闭时构建与 v1.6 字节级一致。
- 新增 `internal/imaging` 包：`Executor` interface + `Transform` 公开 API + `StdlibExecutor`（基于 `image/jpeg` + `image/png` + `image/draw`，纯 stdlib 零依赖）。
- 新增 `parser.ExtractPostImageRefs` / `ExtractPageImageRefs`：扫 Markdown body `![]()` 与 front matter `cover` / `image` / `thumbnail` / `hero`，去重。
- 新增 `generator.imagePipeline` artifact：从 refs 收集 → 解析源文件路径（拒绝外链 / 路径逃逸）→ 调 `imaging.Transform` → 写 `publishDir/{ref}-{width}w.{format}` → 写 `.gobin-images.json` manifest。
- 新增模板 helper `{{ image src alt }}`：emit 原始 `<img>` 让 postprocess 改写为 `<picture>`。
- 扩展 `postprocess.PostprocessHTML` 接收 `ImageSources map[string]ImageSourceRewrite`：把渲染出的 `<img src=...>` 改写为 `<picture><source srcset>` + `<img srcset sizes loading=lazy decoding=async>`，单格式时跳过 `<picture>` 包装直接出 `<img srcset>`。
- 默认 `single.html` 模板加 `{{- if .Post.Params.cover }}` block，零侵入渲染 cover image。

### 体积数据（真实 22 张图）

| 原图 | 480w | 800w | 1200w |
| --- | --- | --- | --- |
| `head/arduino.jpg` 649KB | **30KB (-95%)** | 68KB (-90%) | 151KB (-77%) |
| `head/server.jpg` 477KB | 30KB (-94%) | 73KB (-85%) | 167KB (-65%) |

`enabled: false`（默认）：v1.7 与 v1.6 字节级一致。

### 库 API 新增

- `imaging.Executor` interface
- `imaging.TransformOptions` / `imaging.Variant`
- `imaging.Transform` / `imaging.NewStdlibExecutor`
- `parser.ExtractPostImageRefs` / `parser.ExtractPageImageRefs`
- `parser.ImageRef`

---

## 改进

- Markdown 内容自动参与图片优化（之前只在模板里能引）
- front matter `cover` 字段无需手动调 helper，default `single.html` 已自动渲染
- 解耦的 Executor interface 允许后续接入 WebP/AVIF（disintegration/imaging、libvips）而无需改 Transform
- box filter 缩放（stdlib `image/draw` 缺位时手写）：对 downscale 视觉无差；后续可换 CatmullRom

---

## 兼容性

- 本版本保持配置、模板、CLI 入口、增量构建、并行构建、shortcodes、serve partial rebuild 完全向后兼容。
- `assets.images.enabled: false`（默认）：v1.7 与 v1.6 字节级一致。
- 公开 API 100% 向后兼容；新增 `imaging.*` / `ExtractPostImageRefs` / `ExtractPageImageRefs` / `ImageRef`。
- 模板默认 `single.html` 加了 cover if，模板 trim 标记（`{{- -}}`）保证无 cover 的 post 输出与 v1.6 字节级一致（golden 测试覆盖）。
- 增量构建未参与 image pipeline（每次 build 重新生成所有变体）；下个 patch 接入。

---

## 性能

- v1.7 端到端 clean build（610 篇真实博客 + 22 张图 + image enabled）：约 833ms（v1.6 ~360ms；增量 ~475ms 因新增 22 张图的 transform）
- 关闭 image 优化时（默认）：与 v1.6 字节级一致，零性能损失
- 单图 transform 开销（22 张 480/800/1200 × 2 格式 = 132 个 variants）：~470ms

---

## 验证

发布前执行：

```bash
go test ./...
go test -race ./internal/parser/... ./internal/generator/... ./cmd/gobin/commands/...
go vet ./...
gofmt -l $(find . -name '*.go' -not -path './website/*' -not -path './public/*' -not -path './example-site/*')
make test-coverage
make release-local
```

并手测：

- 真实 610 篇博客 22 张图：config.yaml 加 `assets.images: {enabled: true}`，build 后 `public/img/head/arduino-{480,800,1200}w.{jpg,png}` 应存在；`public/.gobin-images.json` 应有 22 条 entry；引用图的 post HTML 应含 `<picture><source srcset>`。
- 关闭 `assets.images.enabled`：HTML 与产物与 v1.6 字节级一致。
- `gobin check --assets`：v1.5 校验应继续通过（image manifest 不影响 assets manifest）。
- Markdown 单图失败（损坏 JPEG）：build 应输出 WARN 日志并保留原图，不中断构建。
- 增量构建 `gobin build --incremental`：image pipeline 当前不参与（每次全跑），下个 patch 补。
