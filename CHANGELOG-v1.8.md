# Gobin v1.8.0 更新日志

## 发布日期 - 2026-07-07（草稿）

Gobin v1.8.0 引入 **Jekyll 模板兼容层**：自动发现 `_layouts/` 与 `_includes/` 目录，用 front matter 的 `layout:` 字段驱动模板选择，并提供 `{{ .Content }}` 正文注入点。模板语法仍是 Go `html/template`（不引入 Liquid 解释器），但目录结构与布局概念与 Jekyll 对齐，迁移成本大幅降低。

本次发布保持配置、CLI、增量构建、并行构建、图片管线、shortcodes、serve partial rebuild 既有行为完全向后兼容。无 `_layouts/` 的站点字节级一致。

---

## 新增功能

### Jekyll 模板兼容层（v1.8.0）

- **目录发现**：`_layouts/*.html` 与 `_includes/*.html` 自动被 `loadTemplates` 发现，按 basename（去扩展名）注册为模板。`_layouts/post.html` → 模板 `"post"`，`_includes/header.html` → 模板 `"header"`。
- **`layout:` 驱动模板选择**：文章 front matter 的 `layout: custom` 让页面优先使用 `_layouts/custom.html`；无对应布局时回退到现有 `singlePage` / `pagePage`（向后兼容）。
- **正文注入点**：`BasePageData.Content`（`template.HTML`）字段，布局模板用 `{{ .Content }}` 承接正文 HTML（对应 Jekyll `{{ content }}`）。
- **include 调用**：复用现有 `{{ template "name" . }}` 与 `{{ render "name" . }}`，调用 `_includes/` 注册的 partial。

### 设计依据

- 承接 `docs/jekyll-template-compatibility-evaluation.md`（结论：主路线 = 模板迁移，辅路线 = 少量输入兼容，不做 Liquid 运行时）。
- 详细规格：`docs/superpowers/specs/2026-07-07-jekyll-layout-compat-design.md`。
- 迁移指南：`docs/guides/jekyll-layout.md`。

---

## 库 API 变化

- `BasePageData` 新增 `Content template.HTML` 字段（零值 = 空 HTML，不影响现有模板）。
- `Post.Layout` / `Page.Layout` 字段已存在（v1.0 解析期即填充），v1.8 起在渲染期生效。
- `loadTemplates` 新增对 `_layouts/` / `_includes/` 的发现（内部实现 `registerLayoutsAndIncludes`，无新公开导出）。
- 文章/独立页 `PageSpec.TemplateCandidates` 由固定值改为 `[layout, fallback...]`。

---

## 兼容性

- 公开 API 100% 向后兼容（`BasePageData.Content` 是新增字段；`TemplateCandidates` 是内部结构）。
- 无 `_layouts/` / `_includes/` 的站点：注册步骤找不到文件，行为与 v1.7.2 完全一致。
- `layout` 字段为默认值（`"post"` / `"page"`）且无对应布局模板：候选回退到 `singlePage` / `pagePage`，行为与 v1.7.2 一致。
- golden 测试不改动（使用默认模板，无 `_layouts/`）。
- 增量构建 manifest 不受影响（模板文件变化由 templates/ 目录 hash 跟踪，逻辑不变）。

---

## 已知限制 / Deferred

- **不支持 Liquid 语法**：`{% assign %}` / `{% capture %}` / `{% case %}` / 过滤器管道需手动改写为 Go 模板等价逻辑。
- **include 无参数传递**：`{% include x.html foo="bar" %}` 的参数传递不支持。
- **不自动翻译 Jekyll 模板**：需手动按对照表改写（见 `docs/guides/jekyll-layout.md` §3）。
- lossy WebP / AVIF / LQIP / EXIF 保留：仍列入后续版本候选。

---

## 验证

发布前执行：

```bash
go test ./...
go test -race ./internal/parser/... ./internal/generator/... ./internal/imaging/... ./cmd/gobin/commands/...
go vet ./...
gofmt -l internal/ cmd/
go mod tidy
```
