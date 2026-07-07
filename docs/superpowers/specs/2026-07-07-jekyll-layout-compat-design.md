# Jekyll 模板兼容层 — 实现规格 (Spec)

> 日期：2026-07-07
> 状态：草稿，待实现
> 范围：v1.8.0（首个 Jekyll 模板兼容层）
> 承接：`docs/jekyll-template-compatibility-evaluation.md`（结论：主路线 = 模板迁移，辅路线 = 少量输入兼容，不做 Liquid 运行时）

## 1. 问题

当前 Gobin 的模板体系与 Jekyll 的 `_layouts` / `_includes` 体系存在三处对接缺口：

1. **目录不被识别**：Jekyll 站点的 `_layouts/*.html` 与 `_includes/*.html` 不在 `getTemplatePaths` 的发现范围内，构建直接报 `no templates found`。
2. **`layout:` 字段被忽略**：`parser.Post.Layout`（默认 `"post"`）与 `parser.Page.Layout`（默认 `"page"`）已经解析进结构体，但 `pages.go` 的 `TemplateCandidates` 硬编码为 `singlePage` / `listPage` / `pagePage`，front matter 里写 `layout: custom` 完全不生效。
3. **无内容注入点**：Jekyll 布局用 `{{ content }}` 承接子内容；Gobin 数据结构只有 `.Post.ContentHTML`，没有让布局统一引用的 `.Content` 字段。

本规格实现"输入兼容"辅路线（评估文档 §5 推荐）：接受 Jekyll 目录结构 + 布局/content 概念，但模板语法仍是 Go `html/template`（`{{ .Content }}` / `{{ template "x" . }}`），不引入 Liquid 解释器。

## 2. 目标

- `_layouts/`、`_includes/` 目录下的 `.html` 文件被自动发现并按"文件名（去扩展名）"注册为可调用模板，无需 `{{ define }}`。
- `Post.Layout` / `Page.Layout` 驱动页面模板选择：有对应布局模板则用之，否则回退到现有 `singlePage` / `pagePage`（向后兼容）。
- 数据结构提供 `.Content`（`template.HTML`），布局模板用 `{{ .Content }}` 承接正文 HTML。
- 现有默认模板（`templates/_default/*`）、主题（`themes/*/layouts/*`）、golden 测试、增量/并行构建行为完全不变。
- `go test ./...` 持续通过。

## 3. 设计决策

| 决策 | 选择 | 理由 |
|------|------|------|
| 兼容范围 | 目录结构 + layout/content 概念，**不**做 Liquid 语法 | 评估文档明确：Liquid 运行时成本高、收益低、易拖入兼容泥潭；Go 模板已能表达等价结构 |
| 哪些目录走"按文件名注册" | 仅站点级 `_layouts/` 与 `_includes/` | 现有 `templates/` 与主题 `layouts/` 已用 `{{ define }}` 约定，经 `ParseFiles` 注册；混用会双注册冲突 |
| 布局文件名 → 模板名 | 去扩展名的 basename（`_layouts/post.html` → `"post"`） | 与 Jekyll `layout: post` 字段值天然对齐，零映射表 |
| `layout:` 字段未自定义时 | 回退现有模板（`singlePage` / `pagePage`） | 向后兼容：无 `_layouts/` 的站点字节级行为不变 |
| 内容注入方式 | `BasePageData.Content template.HTML` 字段 + `{{ .Content }}` | 单一共享模板集不支持 block 覆盖；字段注入无 race、与并发渲染正交 |
| 多级布局链 | 由模板作者用 `{{ template "base" . }}` 自行组合 | 与现有 `singlePage → base` 链一致，无需新机制 |
| `_includes` 调用方式 | `{{ template "header" . }}` 或 `{{ render "header" . }}` | 复用现有 `render` func，零新 funcMap |

## 4. 目录结构与注册

```
站点根/
  _layouts/        # 新增发现：每个 .html 按 basename 注册
    post.html      # → 模板 "post"
    page.html      # → 模板 "page"
    base.html      # → 模板 "base"
  _includes/       # 新增发现：每个 .html 按 basename 注册
    header.html    # → 模板 "header"
    footer.html    # → 模板 "footer"
  templates/       # 不变：仍走 ParseFiles + {{ define }}
  themes/<t>/layouts/  # 不变：仍走 ParseFiles + {{ define }}
```

注册顺序（避免覆盖现有 `{{ define }}` 模板）：
1. `templates/**` + `themes/<t>/layouts/**` → `ParseFiles`（现有）。
2. `_layouts/*.html` → 逐文件 `tmpl.New(basename).Parse(content)`。
3. `_includes/*.html` → 同上。

basename 注册使用 `New(name).Parse`：无论文件是否含 `{{ define }}`，`Lookup(name)` 都能命中整份文件内容；文件内若有 `{{ define "X" }}`，`X` 亦同时存在，互不冲突。

## 5. 行为契约

### 5.1 layout 字段驱动模板选择

- 文章（post）`TemplateCandidates` 由 `["singlePage"]` 改为 `[post.Layout, "singlePage"]`。
- 独立页（page）由 `["pagePage", "singlePage"]` 改为 `[page.Layout, "pagePage", "singlePage"]`。
- `resolveTemplateName` 仍取首个存在的候选；无对应布局模板时回退不变。

### 5.2 内容注入

- `BasePageData` 新增 `Content template.HTML`。
- 文章页：`Content = template.HTML(post.ContentHTML)`。
- 独立页：`Content = template.HTML(page.ContentHTML)`。
- 列表页 / 404：`Content` 留空（现有模板不引用，无副作用）。
- 布局模板以 `{{ .Content }}` 输出正文；现有默认模板不引用 `.Content`，输出不变。

### 5.3 向后兼容

- 站点无 `_layouts/`、`_includes/` 时：注册步骤 2/3 找不到文件，整体行为与当前完全一致。
- `layout` 字段值无对应模板（如默认 `"post"` 但无 `_layouts/post.html`）：候选回退到 `singlePage`，行为与当前一致。
- golden 测试不改动（使用默认模板，无 `_layouts/`）。

## 6. 不做的事（v1.8.0 边界）

- 不引入 Liquid 解释器（`{% %}` / `{{ }}` Liquid 过滤器）。
- 不自动把 Jekyll Liquid 语法翻译成 Go 模板语法。
- 不改动 `templates/` 与主题 `layouts/` 的注册方式。
- 不承诺"零改动直接运行 Jekyll 站点"——模板需从 Liquid 改写为 Go 模板语法。

## 7. 测试

新增 `internal/generator/layout_compat_test.go`：

1. `TestLayoutDiscovery_RegistersBasenameTemplates`：`_layouts/post.html`、`_includes/header.html` 被注册为 `"post"`、`"header"`，`Lookup` 命中。
2. `TestPostLayout_SelectsLayoutTemplate`：post `layout: custom` + `_layouts/custom.html` → 输出含布局标记。
3. `TestPostLayout_FallsBackToSinglePage`：post 默认 layout、无 `_layouts/` → 走 `singlePage`（向后兼容）。
4. `TestPageLayout_SelectsLayoutTemplate`：page `layout: landing` + `_layouts/landing.html` → 命中。
5. `TestContentInjection_LayoutReceivesContent`：布局 `{{ .Content }}` 输出正文 HTML。
6. `TestIncludes_RenderedInLayout`：布局内 `{{ template "header" . }}` 渲染 `_includes/header.html`。
7. `TestLayoutCompat_BackwardCompatible_NoLayoutsDir`：无 `_layouts/` 时输出与现有 singlePage 一致。

## 8. 文档

- `docs/guides/jekyll-layout.md`：迁移指南（目录约定、`{{ content }}` → `{{ .Content }}`、`{% include x %}` → `{{ template "x" . }}` 对照表）。
- `CHANGELOG-v1.8.md`、`README.md` 更新。
