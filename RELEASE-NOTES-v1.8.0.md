# Gobin v1.8.0 发布说明

## 发布日期 - 2026-07-07

Gobin v1.8.0 引入 **Jekyll 模板兼容层**：自动发现 `_layouts/` 与 `_includes/` 目录，用 front matter 的 `layout:` 字段驱动模板选择，并提供 `{{ .Content }}` 正文注入点。模板语法仍是 Go `html/template`（不引入 Liquid 解释器），但目录结构与布局概念与 Jekyll 对齐，迁移成本大幅降低。

本次发布保持配置、CLI、增量构建、并行构建、图片管线、shortcodes、serve partial rebuild 既有行为完全向后兼容。无 `_layouts/` 的站点字节级一致。

---

## 亮点

- **Jekyll 目录结构兼容**：`_layouts/*.html` 与 `_includes/*.html` 自动被 `loadTemplates` 发现，按 basename（去扩展名）注册为模板。`_layouts/post.html` → 模板 `"post"`，`_includes/header.html` → 模板 `"header"`。
- **`layout:` 驱动模板选择**：文章 front matter 的 `layout: custom` 让页面优先使用 `_layouts/custom.html`；无对应布局时回退到现有 `singlePage` / `pagePage`（向后兼容）。
- **正文注入点**：`BasePageData.Content`（`template.HTML`）字段，布局模板用 `{{ .Content }}` 承接正文 HTML（对应 Jekyll `{{ content }}`）。
- **真实博客验证**：对 608 篇文章的 Beautiful Jekyll 主题博客成功完成端到端迁移验证，生成 730 个页面，31 个静态资源全部正确复制。
- **完全向后兼容**：无 `_layouts/` / `_includes/` 的站点行为与 v1.7.2 字节级一致；公开 API 仅新增 `Content` 字段。

---

## 升级方式

```bash
go install github.com/mengbin92/gobin/cmd/gobin@v1.8.0
```

或从 GitHub Releases 下载对应平台的压缩包，并使用 `SHA256SUMS` 校验。

Docker 用户可以使用：

```bash
docker pull docker.io/mengbin92/gobin:v1.8.0
```

---

## Jekyll 模板兼容层

### 1. 解决什么问题

v1.7.x 时期迁移 Jekyll 站点需要重构整个模板体系（`templates/` + `{{ define }}`），目录约定与 Jekyll 的 `_layouts/` / `_includes/` 不一致，front matter 的 `layout:` 字段也不生效。v1.8.0 提供"输入兼容"路径：保留 Jekyll 目录结构，只需把 Liquid 语法改写为 Go 模板语法。

### 2. 怎么用

在站点根创建 `_layouts/` 和 `_includes/` 目录：

```
站点根/
  _layouts/
    base.html      # 模板 "base"
    post.html      # 模板 "post"
    page.html      # 模板 "page"
  _includes/
    header.html    # 模板 "header"
    footer.html    # 模板 "footer"
```

在布局模板中使用 `{{ .Content }}` 注入正文：

```html
<!-- _layouts/post.html -->
<!DOCTYPE html>
<html>
<head>
  <title>{{ .Title }}</title>
</head>
<body>
  <article>{{ .Content }}</article>
</body>
</html>
```

文章 front matter 的 `layout: post` 会自动选择 `_layouts/post.html`：

```yaml
---
title: My Post
layout: post
---
```

### 3. 语法对照表

| Jekyll (Liquid) | Gobin (Go template) | 说明 |
|-----------------|---------------------|------|
| `{{ content }}` | `{{ .Content }}` | 在布局里插入正文 HTML |
| `{{ page.title }}` | `{{ .Title }}` | 页面标题 |
| `{{ site.title }}` | `{{ .Site.Title }}` | 站点标题 |
| `{% include header.html %}` | `{{ template "header" . }}` | 渲染 include |
| `{% for tag in page.tags %}` | `{{ range .Post.Tags }}` | 遍历标签 |

完整对照表见 `docs/guides/jekyll-layout.md`。

### 4. 测试覆盖

| 测试 | 覆盖 |
|---|---|
| `TestLayoutDiscovery_RegistersBasenameTemplates` | `_layouts/` / `_includes/` 按 basename 注册 |
| `TestLayoutDiscovery_BackwardCompatibleNoLayoutsDir` | 无 Jekyll 目录时不影响现有站点 |
| `TestPostLayout_SelectsLayoutTemplate` | `layout:` 字段正确选择布局 |
| `TestPostLayout_DefaultPostFallsBackToSinglePage` | 默认 layout 回退到 singlePage |
| `TestPageLayout_SelectsLayoutTemplate` | 独立页 layout 选择 |
| `TestIncludes_RenderedInLayout` | include 在布局中正确渲染 |

---

## 库 API 变化

```go
// 新增（v1.8.0）
type BasePageData struct {
    // ... 现有字段 ...
    Content template.HTML  // 正文 HTML（文章页/独立页有值）
}

// 不变
type Post struct { Layout string }  // Layout 字段已存在，v1.8 起生效
type Page struct { Layout string }
```

无新增依赖。

---

## 兼容性说明

- 公开 API 100% 向后兼容（`BasePageData.Content` 是新增字段）。
- 无 `_layouts/` / `_includes/` 的站点：注册步骤找不到文件，行为与 v1.7.2 完全一致。
- `layout` 字段为默认值（`"post"` / `"page"`）且无对应布局模板：候选回退到 `singlePage` / `pagePage`，行为与 v1.7.2 一致。
- golden 测试不改动（使用默认模板，无 `_layouts/`）。
- 增量构建 manifest 不受影响（模板文件变化由 templates/ 目录 hash 跟踪）。
- v1.7.2 WebP 管线不受影响。

---

## 已知限制 / Deferred

- **不支持 Liquid 语法**：`{% assign %}` / `{% capture %}` / `{% case %}` / 过滤器管道需手动改写为 Go 模板等价逻辑。
- **include 无参数传递**：`{% include x.html foo="bar" %}` 的参数传递不支持。
- **不自动翻译 Jekyll 模板**：需手动按对照表改写（见 `docs/guides/jekyll-layout.md`）。
- lossy WebP / AVIF / LQIP / blurhash / EXIF 保留：仍列入后续版本候选。

---

## 集成验证

v1.8.0 对真实博客 `mengbin92.github.io`（Beautiful Jekyll 主题，608 篇文章，7 个 layout，33 个 include）进行了端到端集成验证：

- 730 个页面成功生成（608 文章页 + 60 页分页 + 标签页 + 分类页 + 独立页 + 404 + 首页）
- 文章页正确使用 `_layouts/post.html` 布局
- 未迁移的 Liquid include 被自动跳过并记录 WARN 日志，不影响构建

---

## 验证

发布前建议执行：

```bash
make test
go test -race ./internal/parser/... ./internal/generator/... ./internal/imaging/... ./cmd/gobin/commands/...
go vet ./...
gofmt -l $(find . -name '*.go' -not -path './website/*' -not -path './public/*' -not -path './example-site/*')
go mod tidy
make release-local
```
