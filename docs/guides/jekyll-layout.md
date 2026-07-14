# Jekyll 模板迁移指南

> v1.8.0 起，Gobin 接受 Jekyll 风格的 `_layouts/` 与 `_includes/` 目录，并用 front matter 的 `layout:` 字段驱动模板选择。v1.8.1 起，这两个目录的变化会正确触发增量构建失效和 `serve --watch` 重建。模板语法仍是 Go `html/template`（不是 Liquid），但目录结构与概念与 Jekyll 对齐，迁移成本大幅降低。

## 1. 能做什么 / 不能做什么

| ✅ 现在能做 | ❌ 仍不能做 |
|------------|------------|
| 自动发现 `_layouts/*.html`、`_includes/*.html` | 解释 Liquid 语法（`{% %}`、`{{ | filter }}`） |
| `layout: xxx` 选择 `_layouts/xxx.html` | 自动翻译 Jekyll Liquid 模板 |
| `{{ .Content }}` 注入正文（对应 Jekyll `{{ content }}`） | 承诺"零改动"直接运行原 Jekyll 站点 |
| `{{ template "header" . }}` 调用 include（对应 `{% include header.html %}`） | |

迁移的本质是：**保留目录结构和布局概念，把 Liquid 语法手动改写成 Go 模板语法**。不需要重新设计模板体系。

## 2. 目录约定

```
站点根/
  _layouts/        # 每个文件按 basename 注册为模板
    base.html      #   → 模板 "base"
    post.html      #   → 模板 "post"
    page.html      #   → 模板 "page"
  _includes/       # 每个文件按 basename 注册为模板
    header.html    #   → 模板 "header"
    footer.html    #   → 模板 "footer"
  templates/       # Gobin 原生模板，走 {{ define }} 约定（不变）
  themes/<t>/layouts/   # 主题模板，走 {{ define }} 约定（不变）
```

**注册顺序**：`templates/` 与主题 `layouts/` 先加载（`{{ define }}`），`_layouts/` 与 `_includes/` 后加载（按文件名）。若 basename 与已有模板名冲突，**已有模板优先**（不会被 `_layouts/` 覆盖）。

## 3. 语法对照表

| Jekyll (Liquid) | Gobin (Go template) | 说明 |
|-----------------|---------------------|------|
| `{{ content }}` | `{{ .Content }}` | 在布局里插入正文 HTML |
| `{{ page.title }}` | `{{ .Title }}` | 页面标题 |
| `{{ page.description }}` | `{{ .Description }}` | 描述 |
| `{{ site.title }}` | `{{ .Site.Title }}` | 站点标题 |
| `{{ site.author }}` | `{{ .Site.Author }}` | 作者 |
| `{{ site.baseurl }}` | `{{ .Site.BaseURL }}` | base URL |
| `{% include header.html %}` | `{{ template "header" . }}` | 渲染 include（basename 无扩展名） |
| `{% include header.html type="post" %}` | `{{ template "header" . }}` | 参数传递需在 include 内用 `.Site` / `.Title` 间接判断，不支持 Liquid 参数 |
| `{{ page.date | date: "%Y-%m-%d" }}` | `{{ dateFormat "2006-01-02" .Post.Date }}` | Go 时间格式 |
| `{{ page.url }}` | `{{ .Post.URL }}` | 文章 URL |
| `{{ post.excerpt }}` | `{{ .Summary }}` 或 `{{ safeHTML .SummaryHTML }}` | 摘要 |
| `{% for tag in page.tags %}` | `{{ range .Post.Tags }}` | 遍历标签 |
| `{% for post in paginator.posts %}` | `{{ range .Posts }}` | 遍历列表文章 |
| `{{ paginator.previous_page_path }}` | `{{ .Pagination.PrevPage }}` | 分页（见下） |

## 4. 数据模型

每个模板可用的数据字段（以 `.` 开头）：

### 通用（所有页面）

| 字段 | 类型 | 说明 |
|------|------|------|
| `.Site` | `*config.Config` | 站点配置（`.Title` / `.Author` / `.BaseURL` / `.Description` / `.Social` ...） |
| `.Title` | `string` | 页面标题 |
| `.Description` | `string` | 描述 |
| `.Canonical` | `string` | 规范 URL |
| `.Content` | `template.HTML` | 正文 HTML（文章页/独立页有值，列表页/404 为空） |

### 文章页（`SinglePageData`）

| 字段 | 说明 |
|------|------|
| `.Post` | `*parser.Post`（`.Title` / `.Date` / `.Tags` / `.Categories` / `.ContentHTML` / `.URL` / `.Params`） |
| `.PrevPost` / `.NextPost` | 上一篇/下一篇 |

### 列表页（`ListPageData`）

| 字段 | 说明 |
|------|------|
| `.Posts` | 当前页文章切片 |
| `.Pagination.Page` / `.TotalPages` / `.TotalPosts` | 分页信息 |
| `.Pagination.PrevPage` / `.NextPage` | 上一页/下一页页码 |
| `.Pagination.IsFirstPage` / `.IsLastPage` | 边界标志 |

### 独立页（`StandalonePageData`）

| 字段 | 说明 |
|------|------|
| `.Page` | `*parser.Page`（`.Title` / `.ContentHTML` / `.URL`） |

## 5. 分页导航示例

Jekyll：

```liquid
{% if paginator.previous_page %}
<a href="{{ paginator.previous_page_path }}">← Newer</a>
{% endif %}
```

Gobin：

```gotemplate
{{ if and (gt .Pagination.PrevPage 0) (not .Pagination.IsFirstPage) }}
  {{ if eq .Pagination.PrevPage 1 }}
  <a href="/">← Newer</a>
  {{ else }}
  <a href="/{{ $.Site.PaginatePath }}/{{ .Pagination.PrevPage }}/">← Newer</a>
  {{ end }}
{{ end }}
```

## 6. 迁移步骤

1. **建目录**：在站点根创建 `_layouts/` 和 `_includes/`（若沿用 Jekyll 结构则已存在）。
2. **选布局**：把 Jekyll `_layouts/base.html` 作为最外层布局，把 `{{ content }}` 改成 `{{ .Content }}`。
3. **改 include**：把 `{% include xxx.html %}` 改成 `{{ template "xxx" . }}`。
4. **设 layout**：文章 front matter 保持 `layout: post`；`_layouts/post.html` 包裹正文并 `{{ template "base" . }}` 指向最外层。
5. **逐页验证**：先跑首页（列表页），再跑文章页，最后补 taxonomy / 404。
6. **图片/资源**：路径保持 `/assets/...` 或 `/css/...`，与 v1.7 图片管线正交。

## 7. 局限性（v1.8.0）

- **不支持 Liquid**：`{% assign %}` / `{% capture %}` / `{% case %}` / 过滤器管道（`| strip_html` / `| truncatewords` 等）需手动改写为 Go 模板等价逻辑。
- **include 无参数**：`{% include x.html foo="bar" %}` 的 Liquid 参数传递不支持；include 内只能访问调用方传入的同一数据上下文（`.`）。
- **不自动翻译**：需手动把 Liquid 模板改写为 Go 模板语法（对照表见 §3）。
- **basename 冲突**：若 `_layouts/header.html` 与 `templates/partials/header.html` 同时存在且后者用 `{{ define "header" }}`，后者优先。

## 8. 排错

| 症状 | 原因 | 处理 |
|------|------|------|
| `no template found for candidates: post,singlePage` | `layout: post` 但无 `_layouts/post.html`，且 singlePage 也没加载 | 确认 `templates/_default/single.html` 存在；或在 `_layouts/post.html` 补文件 |
| 布局输出了但 `{{ .Content }}` 为空 | 数据未注入 Content | 确认走的是 `SinglePageData` / `StandalonePageData`（文章/独立页才有 Content） |
| include 不渲染 | basename 未命中或写法错误 | 用 `{{ template "header" . }}`（不是 `{{ include "header" }}`）；确认 `_includes/header.html` 存在 |
