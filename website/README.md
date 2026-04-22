# Gobin 官方网站

这是 Gobin 项目的官方网站，使用 Gobin 本身构建。

## 🌐 在线预览

访问 [https://gobin.dev](https://gobin.dev) 查看在线版本（部署后）。

## 🚀 本地开发

### 快速启动

```bash
# 1. 构建 gobin（如果还没构建）
cd /path/to/gobin
go build -o gobin ./cmd/gobin

# 2. 启动开发服务器
cd website
../gobin serve
```

访问 http://localhost:8080 查看效果。

### 开发模式

```bash
# 启用文件监听和自动重载
../gobin serve --watch
```

修改文件后，Gobin 会自动检测变更并重新构建；浏览器需要手动刷新查看最新内容。

### 构建静态文件

```bash
# 普通构建
../gobin build

# 轻量压缩构建（生产环境推荐）
../gobin build --minify
```

`--minify` 当前会对 HTML 和 CSS 做保守压缩，JavaScript 内容保持原样，适合先保证站点输出安全稳定。

生成的文件在 `public/` 目录。

## 📁 目录结构

```
website/
├── config.yaml           # 网站配置
├── _posts/               # 页面内容
│   ├── index.md          # 首页
│   ├── about.md          # 关于页面
│   ├── docs.md           # 文档索引
│   ├── getting-started.md    # 快速开始
│   ├── configuration.md      # 配置指南
│   └── deployment.md         # 部署指南
├── assets/               # 静态资源
│   └── images/           # 图片（favicon、OG 图片等）
└── public/               # 构建输出（自动生成）
```

## 🎨 主题

网站使用 `themes/official-website` 主题，包含：

- 深色主题（默认）+ 浅色主题切换
- 黑客风格设计（等宽字体、终端演示）
- 响应式布局（移动端友好）
- SEO 优化（Open Graph、Twitter Card）
- 代码高亮

## 📝 添加新页面

在 `_posts/` 目录创建新的 Markdown 文件：

```yaml
---
title: "页面标题"
date: 2026-01-10T00:00:00+08:00
description: "页面描述"
draft: false
categories: ["docs"]
tags: ["tag1", "tag2"]
---

# 内容

页面内容...
```

## 🚢 自动部署

网站配置了 GitHub Actions 自动部署到 GitHub Pages。

### 触发条件

- 推送到 `main` 分支
- 修改以下路径的文件：
  - `website/**`
  - `themes/official-website/**`
  - `.github/workflows/website.yml`

### 手动触发

在 GitHub Actions 页面点击 "Run workflow" 按钮。

## 📊 性能

- ⚡ 构建时间：~20ms
- 📦 首次内容绘制：< 1s
- 🎯 SEO 得分：> 95

## 🔧 自定义配置

### 基本配置

编辑 `config.yaml`：

```yaml
title: "Gobin"
description: "高性能静态博客生成器"
baseURL: "https://gobin.dev"
theme: "official-website"
```

### 导航配置

```yaml
navbarLinks:
  - name: "Features"
    url: "/#features"
  - name: "Quick Start"
    url: "/#quickstart"
  - name: "Docs"
    url: "/docs/"
```

### SEO 配置

```yaml
seo:
  enabled: true
  openGraph: true
  twitterCard: true
  image: "/images/og-image.svg"
```

## 🌐 部署到其他平台

### Vercel

创建 `vercel.json`：

```json
{
  "buildCommand": "gobin build --minify",
  "outputDirectory": "public"
}
```

### Netlify

创建 `netlify.toml`：

```toml
[build]
  command = "gobin build --minify"
  publish = "public"
```

## 📦 静态资源

### Favicon

- SVG 格式：`/images/favicon.svg`
- Apple Touch Icon：`/images/apple-touch-icon.png`

### OG 图片

- SVG 格式：`/images/og-image.svg`
- 用于社交分享预览

## 🔗 相关链接

- [Gobin GitHub](https://github.com/mengbin92/gobin)
- [Gobin 文档](https://github.com/mengbin92/gobin/blob/main/README.md)
- [主题开发指南](/docs/themes/)

## 📄 许可证

MIT License - 与 Gobin 主项目相同
