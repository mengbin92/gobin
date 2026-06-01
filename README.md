# Gobin - 高性能静态博客生成器

Gobin 是一个基于 Go 语言开发的静态博客网站生成器，专为追求极致性能和高定制性的博客作者设计。它兼容常见的 Jekyll 博客内容结构，让您能够迁移现有 Markdown 内容，同时享受更快的构建速度和更灵活的定制能力。

## 项目背景

本项目旨在将现有的 Jekyll 博客（使用 Beautiful Jekyll 主题）迁移到一个自研的静态网站生成器上。通过使用 Go 语言重写，我们期望达到以下目标：

- **极速构建**：相比 Jekyll，构建速度提升 10-100 倍
- **零依赖部署**：单二进制文件，无需运行时环境
- **迁移友好**：保持常见 Markdown 文件结构和 Front Matter 格式
- **灵活定制**：强大的模板系统和配置选项

## 当前能力

### 已支持
- Markdown + YAML Front Matter 解析
- `_posts/` 目录结构
- 列表页、文章页、标签页、分类页生成
- RSS、Atom、Sitemap、搜索索引生成
- 基础主题模板与静态资源复制
- `build`、`serve`、`init`、`new`、`check`、`version` CLI 命令
- `permalinks.posts` 文章链接配置
- `draft` / `published` 内容可见性控制
- `assets.fingerprint` 静态资源指纹（`query` / `filename` 两种策略）
- `serve` watch 模式下的 LiveReload 注入
- `gobin build --incremental` 增量构建：按 source / list / feed / search / sitemap 多类别指纹跳过未变化的产物
- `gobin serve` watcher 重建自动启用增量，只重渲染受影响的产物
- `gobin build --jobs N` 并行页面渲染：多 worker 并发渲染文章/列表/taxonomy 页面，与增量构建正交叠加
- Hugo 风格短代码（shortcodes）：`{{< name args >}}` / `{{% name args %}}`，内置 `figure` / `youtube` / `gist` / `highlight`，并支持站点与主题自定义

### 当前限制
- 多语言、图片优化等能力仍在规划中
- 并行仅覆盖页面渲染阶段；feed/sitemap/search 等聚合产物与 Markdown 解析仍为串行

### 规划中
- 更完整的 Jekyll 迁移兼容层
- 多语言支持
- 更完善的主题系统和开发服务器体验
- 图片与资源优化

## 技术栈

- **后端生成器**：Go 1.25
- **Markdown 渲染**：[goldmark](https://github.com/yuin/goldmark)
- **代码高亮**：[Chroma](https://github.com/alecthomas/chroma) / Prism.js
- **模板引擎**：Go `html/template`
- **CLI 框架**：[cobra](https://github.com/spf13/cobra)
- **CSS 框架**：Tailwind CSS（可选）

## 快速开始

### 安装

```bash
# 从源码安装（推荐）
git clone https://github.com/mengbin92/gobin.git
cd gobin
go build -o gobin ./cmd/gobin

# 使用 Go install 安装
go install github.com/mengbin92/gobin/cmd/gobin@latest

# 使用 Docker 运行
docker run --rm -p 8080:8080 \
  -v "$PWD:/site" \
  docker.io/mengbin92/gobin:latest
```

### 创建新站点

```bash
# 初始化新博客
gobin init my-blog
cd my-blog

# 目录结构
my-blog/
├── _posts/           # 博客文章
├── assets/           # 静态资源
├── templates/        # 页面模板
├── config.yaml       # 配置文件
└── public/           # 构建输出（自动生成）
```

### 创建第一篇文章

```bash
# 创建文章文件
$EDITOR _posts/2026-01-04-my-first-post.md
```

文章格式：
```markdown
---
title: "我的第一篇文章"
date: 2026-01-04T10:00:00+08:00
description: "这是我的博客第一篇文章"
tags: ["博客", "教程"]
categories: ["生活"]
draft: false
---

# 文章内容

## 开始写作

从这里开始你的写作之旅...
```

### 本地预览

```bash
# 启动开发服务器
gobin serve

# 或指定端口
gobin serve -p 8080

# 启用文件监听和自动刷新
gobin serve --watch
```

访问 http://localhost:8080 查看你的博客。

### 构建站点

```bash
# 构建静态文件
gobin build

# 增量构建，仅重写受影响的产物
gobin build --incremental --clean=false

# 轻量压缩输出（保守模式）
gobin build --minify

# 包含草稿文章
gobin build --drafts

# 跳过输出目录清理
gobin build --clean=false

# 控制并行页面渲染的 worker 数（0 = 自动，1 = 串行）
gobin build --jobs 4
```

`--minify` 当前会对 HTML 和 CSS 做保守压缩，并保留 JavaScript 原始内容，优先保证输出正确性而不是做激进资源改写。

`--jobs` 控制并行渲染页面的 worker 数：`0`（默认）按 CPU 数自动选择并封顶为 4，`1` 强制串行。页面渲染以写入大量小文件为主、偏 I/O 密集，因此默认封顶可在多核机器上获得收益（基准约 15–19%）而不引入高并发下的文件系统竞争退化；如使用更重、更偏 CPU 的模板，可显式指定更大的 `--jobs`。并行只改变写盘顺序、不改变内容，产物与串行字节级一致，且可与 `--incremental` 叠加。

使用 `--clean=false` 时，未变化的静态资源会跳过复制以加快重建。Gobin 会通过资源 manifest 清理上次构建记录过、但本次源目录中已不存在的旧静态资源；未被资源管线管理的输出文件仍会保留。

## 配置说明

### 主配置文件（config.yaml）

```yaml
# 网站基本信息
title: 我的个人博客
description: 专注于技术分享和生活记录
theme: default
languageCode: zh-CN
baseURL: https://example.github.io

# 目录配置
contentDir: _posts
staticDir: assets
publishDir: public

# 分页配置
paginate: 10
paginatePath: page

# Permalink 配置
permalinks:
  posts: /:year-:month-:day-:title/

# 导航链接
navbarLinks:
  - name: 首页
    url: /
  - name: 分类
    url: /categories/
  - name: 标签
    url: /tags/
  - name: 关于
    url: /about/

# 社交媒体
social:
  github: your-github-username
  email: your-email@example.com

# 功能开关
enableEmoji: true
enableGitInfo: true
enableRobotsTXT: true

# 站点级产物开关（可选）
outputs:
  feed: true
  search: true
  sitemap: true
  robots: true

# 评论系统（可选）
comments:
  enabled: false
  provider: utterances
  utterances:
    repo: "username/repo"
    theme: "github-light"

# Markdown 渲染和代码高亮
markup:
  # 默认关闭 Markdown 中的原始 HTML。
  # 迁移可信旧内容且需要 HTML 片段时，可显式设为 true。
  allowUnsafeHTML: false
  highlight:
    style: github
    lineNos: true
```

`markup.allowUnsafeHTML` 默认关闭。显式设置为 `true` 时，Markdown 中的原始 HTML 会作为活动 HTML 输出，适合迁移完全可信的旧内容；内容来源不完全可信时应保持默认值。

## 短代码（Shortcodes）

短代码让你在 Markdown 正文里用简短指令生成结构化 HTML，而不必开启全局 `allowUnsafeHTML` 或手写重复 HTML。语法兼容 Hugo：

| 形式 | 说明 |
|------|------|
| `{{< name args >}}` | HTML 形式，模板输出作为**原始 HTML** 注入（即使 `allowUnsafeHTML: false` 也生效）|
| `{{% name args %}}` | Markdown 形式，模板输出**再经 Markdown 渲染** |
| `{{< name >}}body{{< /name >}}` | 配对形式，正文通过 `.Inner` 提供 |

参数支持位置参数与引号命名参数，二者可混用：

```markdown
{{< youtube dQw4w9WgXcQ >}}

{{< figure src="/img/cover.png" alt="封面" caption="图 1" >}}

{{< highlight go >}}
fmt.Println("hello")
{{< /highlight >}}
```

### 内置短代码

| 名称 | 参数 | 用途 |
|------|------|------|
| `figure` | `src`（必填）、`alt`、`title`、`caption`、`link` | 输出 `<figure>` 图片块 |
| `youtube` | 视频 id（位置或 `id=`） | 响应式 YouTube 嵌入 |
| `gist` | `user`、`id`（位置或命名） | 嵌入 GitHub Gist |
| `highlight` | 语言（位置 0），配对 | 包裹正文为代码块 |

### 自定义短代码

新增或覆盖短代码：在站点 `templates/shortcodes/<name>.html` 放一个 Go 模板即可（主题作者用 `<theme>/layouts/shortcodes/<name>.html`）。覆盖优先级为**站点 > 主题 > 内置**，与模板覆盖规则一致。

模板上下文提供：

- `{{ .Get 0 }}`：第 N 个位置参数；`{{ .Get "key" }}`：命名参数（缺失返回空串）
- `{{ .Inner }}`：配对短代码的正文（已先行展开嵌套短代码；按文本转义，需原始 HTML 用 `{{ .Inner | safeHTML }}`）
- `{{ .Name }}`：短代码名称
- 辅助函数：`safeHTML`、`absURL`、`urlize`、`default`

示例 `templates/shortcodes/note.html`：

```html
<div class="note note-{{ .Get "type" | default "info" }}">{{ .Inner | safeHTML }}</div>
```

### 说明

- 代码围栏（```` ``` ````）与行内代码（`` `...` ``）中的短代码语法不会展开。
- 引用未注册的短代码会**中断构建并指出文件与名称**，便于及早发现拼写错误。
- 选型建议：要输出 HTML 用 `{{< >}}`，要输出仍走 Markdown 渲染的内容用 `{{% %}}`。短代码改动会触发增量构建失效与 `serve` 全量重载。

## 项目结构

```
gobin/
├── cmd/
│   └── gobin/          # CLI 入口
├── internal/
│   ├── config/         # 配置管理
│   ├── parser/         # 内容解析
│   └── generator/      # 站点生成
├── templates/          # 默认模板
├── assets/             # 默认静态资源
├── themes/             # 主题目录
├── docs/               # 文档
├── examples/           # 示例站点
└── scripts/            # 迁移和工具脚本
```

## CLI 命令参考

| 命令 | 说明 | 示例 |
|------|------|------|
| `gobin init [name]` | 初始化新站点 | `gobin init myblog` |
| `gobin new <post|page> <title>` | 创建文章或页面草稿 | `gobin new post "Release notes"` |
| `gobin check` | 校验配置、内容、模板和 permalink 冲突 | `gobin check --drafts` |
| `gobin build` | 构建静态站点，可选启用增量构建、并行渲染和保守 HTML/CSS 压缩 | `gobin build --incremental --clean=false --jobs 4 --minify` |
| `gobin serve` | 启动开发服务器 | `gobin serve -p 8080 --drafts --clean=false` |
| `gobin version` | 显示版本信息 | `gobin version` |
| `gobin help` | 显示帮助信息 | `gobin help` |

## 从 Jekyll 迁移

### 迁移步骤

```bash
# 1. 备份现有 Jekyll 博客
git clone https://github.com/username/username.github.io.git blog-backup

# 2. 创建新站点
gobin init my-new-blog
cd my-new-blog

# 3. 复制内容
cp -r ../blog-backup/_posts ./
cp -r ../blog-backup/assets ./

# 4. 转换配置（使用转换脚本）
./scripts/convert-jekyll-config.py ../blog-backup/_config.yml > config.yaml

# 5. 构建测试
gobin build
gobin serve

# 6. 验证内容
# - 检查文章列表
# - 验证文章详情页
# - 测试标签和分类
# - 检查永久链接
```

### 兼容性问题处理

#### URL 重定向
如需保持旧 URL 可访问，在文章 Front Matter 中添加：

```yaml
---
title: "旧文章标题"
aliases:
  - /old-url/
  - /another-old-url/
---
```

#### 模板语法转换

| Jekyll Liquid | Go Template |
|--------------|-------------|
| `{{ page.title }}` | `{{ .Title }}` |
| `{% for post in paginator.posts %}` | `{{ range .Paginator.Pages }}` |
| `{% if page.tags %}` | `{{ with .Tags }}` |
| `{{ post.excerpt }}` | `{{ .Summary }}` |
| `{{ site.baseurl }}` | `{{ .Site.BaseURL }}` |

## 部署方案

### GitHub Pages 部署

使用 GitHub Actions 自动化部署：

```yaml
# .github/workflows/deploy.yml
name: Deploy to GitHub Pages

on:
  push:
    branches: [ main ]

jobs:
  deploy:
    runs-on: ubuntu-latest
    permissions:
      pages: write
      id-token: write
    steps:
    - uses: actions/checkout@v4
    - uses: actions/setup-go@v5
      with:
        go-version-file: go.mod
    - run: go install github.com/mengbin92/gobin/cmd/gobin@latest
    - run: gobin build --minify
    - uses: actions/upload-pages-artifact@v2
      with:
        path: ./public
    - uses: actions/deploy-pages@v2
```

### Vercel 部署

```json
// vercel.json
{
  "buildCommand": "gobin build --minify",
  "outputDirectory": "public",
  "framework": null,
  "cleanUrls": true
}
```

### Netlify 部署

```toml
# netlify.toml
[build]
  command = "gobin build --minify"
  publish = "public"

[build.environment]
  GO_VERSION = "1.25"
```

### Docker 部署

官方镜像发布到 Docker Hub，支持 `linux/amd64` 和 `linux/arm64`：

```bash
docker pull docker.io/mengbin92/gobin:latest

docker run --rm -p 8080:8080 \
  -e GOBIN_AUTO_INIT=true \
  -v "$PWD:/site" \
  docker.io/mengbin92/gobin:latest
```

也可以使用 `docker-compose.yml`：

```bash
GOBIN_IMAGE=docker.io/mengbin92/gobin:latest docker compose up
```

## 性能指标

### 性能基线

项目提供可重复运行的 Go benchmark 入口，用于跟踪解析、配置加载、分页和静态资源复制等核心路径：

```bash
make benchmark
```

该命令会运行 `go test -run '^$' -bench=. -benchmem -count=1 ./...`，并将结果写入 `benchmark-results.txt`。CI 会在每次 push 和 pull request 中运行同一入口，并上传 benchmark 结果作为构建产物。

### 输出优化
- `--minify` 支持对 HTML 和 CSS 做保守压缩
- 静态资源在 `--clean=false` 重建时会跳过未变化文件，并通过资源 manifest 清理上次构建记录过但本次已移除的旧资源
- RSS、Atom、Sitemap、搜索索引用于改善站点可发现性

## 开发计划

### 第一阶段：核心功能（1-2周）
- [x] 项目基础架构搭建
- [x] CLI 命令行接口
- [x] 内容解析器（Front Matter + Markdown）
- [x] HTML 模板引擎集成
- [x] 基础站点生成逻辑
- [x] 单篇文章和列表页面生成

### 第二阶段：进阶功能（2-3周）
- [x] 标签和分类系统
- [x] 分页功能实现
- [x] RSS/Atom Feed 生成
- [x] Sitemap.xml 生成
- [x] 搜索索引生成
- [x] 开发服务器（静态文件服务 + 文件监听自动重建）

### 第三阶段：功能完善（2周）
- [x] 主题系统实现
- [x] SEO 基础产物（Sitemap、Feed、robots.txt、canonical 数据）
- [x] 评论和分析模板占位
- [ ] 多语言支持
- [ ] 图片优化

### 第四阶段：测试与优化（1-2周）

**状态**：持续优化中
- [x] 单元测试编写
- [x] 集成测试
- [x] 静态资源复制优化
- [x] CI benchmark 基线
- [x] 增量构建
- [x] 并行构建
- [ ] 指纹资源和更完整的资源管线
- [ ] 文档持续收口
- [x] 示例站点创建

## 贡献指南

欢迎贡献代码、提出问题或建议！

1. Fork 本仓库
2. 创建特性分支（`git checkout -b feature/AmazingFeature`）
3. 提交更改（`git commit -m 'Add some AmazingFeature'`）
4. 推送到分支（`git push origin feature/AmazingFeature`）
5. 开启 Pull Request

## 许可证

本项目采用 MIT 许可证。详见 [LICENSE](LICENSE) 文件。

## 参考资源

- [Hugo](https://gohugo.io/) - 另一个流行的 Go 静态网站生成器
- [Zola](https://getzola.org/) - Rust 编写的静态博客生成器
- [Jekyll](https://jekyllrb.com/) - Ruby 静态网站生成器
- [Beautiful Jekyll](https://beautifuljekyll.com/) - 当前 Jekyll 主题

## 联系方式

- **作者**：孟斯特
- **邮箱**：mengbin1992@outlook.com
- **GitHub**：[@mengbin92](https://github.com/mengbin92)

---

**注意**：本项目目前处于开发阶段，部分功能可能尚未实现。查看 [开发计划](#开发计划) 了解当前进度。
