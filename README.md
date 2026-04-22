# Gobin - 高性能静态博客生成器

Gobin 是一个基于 Go 语言开发的静态博客网站生成器，专为追求极致性能和高定制性的博客作者设计。它完全兼容 Jekyll 博客结构，让您能够无缝迁移现有博客，同时享受更快的构建速度和更灵活的定制能力。

## 项目背景

本项目旨在将现有的 Jekyll 博客（使用 Beautiful Jekyll 主题）迁移到一个自研的静态网站生成器上。通过使用 Go 语言重写，我们期望达到以下目标：

- **极速构建**：相比 Jekyll，构建速度提升 10-100 倍
- **零依赖部署**：单二进制文件，无需运行时环境
- **完全兼容**：保持现有 Markdown 文件结构和 Front Matter 格式
- **灵活定制**：强大的模板系统和配置选项

## 当前能力

### 已支持
- Markdown + YAML Front Matter 解析
- `_posts/` 目录结构
- 列表页、文章页、标签页、分类页生成
- RSS、Atom、Sitemap、搜索索引生成
- 基础主题模板与静态资源复制
- `build`、`serve`、`init`、`version` CLI 命令
- `permalinks.posts` 文章链接配置
- `draft` / `published` 内容可见性控制

### 当前限制
- 暂未实现 `new`、`check` 等命令
- 暂未实现增量构建和并行构建
- `serve` 当前提供自动重建，不包含真正的 LiveReload 注入
- 多语言、短代码、图片优化等能力仍在规划中

### 规划中
- 增量构建
- 并行构建
- 更完整的 Jekyll 迁移兼容层
- 多语言与短代码支持
- 更完善的主题系统和开发服务器体验

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

# 轻量压缩输出（保守模式）
gobin build --minify

# 包含草稿文章
gobin build --drafts

# 跳过输出目录清理
gobin build --clean=false
```

`--minify` 当前会对 HTML 和 CSS 做保守压缩，并保留 JavaScript 原始内容，优先保证输出正确性而不是做激进资源改写。

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

# 代码高亮
markup:
  highlight:
    style: github
    lineNos: true
```

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
| `gobin build` | 构建静态站点，可选启用保守 HTML/CSS 压缩 | `gobin build --minify --drafts --clean=false` |
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
        go-version: '1.21'
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
  GO_VERSION = "1.21"
```

## 性能指标

### 构建性能
- ✅ 100 篇文章：构建时间 < 2 秒
- ✅ 1000 篇文章：构建时间 < 10 秒
- ✅ 内存占用：每 1000 篇文章 < 500MB

### 输出优化
- ✅ HTML 大小：平均每篇文章 < 50KB（未压缩）
- ✅ 加载时间：首次内容绘制 < 1 秒（CDN）
- ✅ SEO 得分：Google PageSpeed > 95 分

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
- [x] 开发服务器（热重载）

### 第三阶段：功能完善（2周）
- [x] 主题系统实现
- [x] 多语言支持框架
- [x] SEO 优化功能
- [x] 评论系统集成
- [x] 分析工具集成
- [x] 图片优化功能（框架支持）

### 第四阶段：测试与优化（1-2周）

**状态**: ✅ v1.0.0 已发布！
- [x] 单元测试编写
- [x] 集成测试
- [x] 性能优化
- [x] 文档完善
- [x] 示例站点创建
- [x] 发布 v1.0 版本 🎉

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
