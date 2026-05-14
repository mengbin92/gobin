package commands

import (
	"fmt"
	"path/filepath"
	"time"
)

func scaffoldDirectories() []string {
	return []string{
		"_posts",
		"assets",
		"assets/css",
		"assets/js",
		"templates",
		"templates/_default",
		"templates/partials",
	}
}

func scaffoldFiles(siteName string) map[string]string {
	postDate := time.Now().Format("2006-01-02")

	return map[string]string{
		"config.yaml":                                   scaffoldConfig(siteName),
		".gitignore":                                    scaffoldGitignore(),
		"assets/css/main.css":                           scaffoldDefaultCSS(),
		"templates/_default/base.html":                  scaffoldBaseTemplate(),
		"templates/_default/list.html":                  scaffoldListTemplate(),
		"templates/_default/single.html":                scaffoldSingleTemplate(),
		"templates/_default/taxonomy.html":              scaffoldTaxonomyTemplate(),
		"templates/_default/404.html":                   scaffold404Template(),
		"templates/partials/header.html":                scaffoldHeaderTemplate(),
		"templates/partials/footer.html":                scaffoldFooterTemplate(),
		filepath.Join("_posts", postDate+"-welcome.md"): scaffoldSamplePost(siteName),
	}
}

func scaffoldConfig(name string) string {
	return fmt.Sprintf(`# 网站基本信息
title: %s
description: 这是一个使用 Gobin 构建的静态博客
languageCode: zh-CN
baseURL: /

# 作者信息
author: Your Name
email: your-email@example.com

# 目录配置
contentDir: _posts
staticDir: assets
publishDir: public
themesDir: themes

# 分页配置
paginate: 10
paginatePath: page

# Permalink 配置
permalinks:
  posts: /:year/:month/:day/:title/

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

# Markdown 渲染
markup:
  allowUnsafeHTML: false

# 产物开关（可选）
outputs:
  feed: true
  search: true
  sitemap: true
  robots: true
`, name)
}

func scaffoldBaseTemplate() string {
	return `{{ define "base" }}
<!DOCTYPE html>
<html lang="{{ .Site.LanguageCode }}">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>{{ if .Title }}{{ .Title }} | {{ .Site.Title }}{{ else }}{{ .Site.Title }}{{ end }}</title>
    <meta name="description" content="{{ if .Description }}{{ .Description }}{{ else }}{{ .Site.Description }}{{ end }}">
    {{ if .Canonical }}<link rel="canonical" href="{{ .Canonical }}">{{ end }}
    <link rel="stylesheet" href="{{ stylesheetPath }}">
</head>
<body>
    {{ render .HeaderTemplate . }}

    <main class="container">
        {{ render .MainTemplate . }}
    </main>

    {{ render .FooterTemplate . }}
</body>
</html>
{{ end }}
`
}

func scaffoldListTemplate() string {
	return `{{ define "listPage" }}
{{ template "base" . }}
{{ end }}

{{ define "listMain" }}
<div class="post-list">
    <h1>{{ .Title }}</h1>
    <ul>
        {{ range .Posts }}
        <li>
            <span class="date">{{ .Date.Format "2006-01-02" }}</span>
            <a href="{{ .URL }}">{{ .Title }}</a>
        </li>
        {{ end }}
    </ul>

    {{ if .Pagination }}
    <nav class="pagination">
        <div class="pagination-info">
            Page {{ .Pagination.Page }} of {{ .Pagination.TotalPages }}
        </div>
        <div class="pagination-links">
            {{ if and (gt .Pagination.PrevPage 0) (not .Pagination.IsFirstPage) }}
                {{ if eq .Pagination.PrevPage 1 }}
                <a href="/" class="pagination-prev">← 上一页</a>
                {{ else }}
                <a href="/{{ $.Site.PaginatePath }}/{{ .Pagination.PrevPage }}/" class="pagination-prev">← 上一页</a>
                {{ end }}
            {{ end }}

            {{ if and (gt .Pagination.NextPage 0) (not .Pagination.IsLastPage) }}
            <a href="/{{ $.Site.PaginatePath }}/{{ .Pagination.NextPage }}/" class="pagination-next">下一页 →</a>
            {{ end }}
        </div>
    </nav>
    {{ end }}
</div>
{{ end }}
`
}

func scaffoldSingleTemplate() string {
	return `{{ define "singlePage" }}
{{ template "base" . }}
{{ end }}

{{ define "singleMain" }}
<article class="post">
    <header>
        <h1>{{ .Post.Title }}</h1>
        <div class="meta">
            <span class="date">{{ .Post.Date.Format "2006-01-02" }}</span>
            {{ with .Post.Tags }}
            <span class="tags">Tags: {{ range . }}{{ . }} {{ end }}</span>
            {{ end }}
        </div>
    </header>
    <div class="content">
        {{ .Post.ContentHTML | safeHTML }}
    </div>
</article>
{{ end }}
`
}

func scaffold404Template() string {
	return `{{ define "notFoundMain" }}
<div class="error-page">
    <h1>404 - Page Not Found</h1>
    <p>The page you're looking for doesn't exist.</p>
    <a href="/">Go back home</a>
</div>
{{ end }}

{{ define "notFoundPage" }}
{{ template "base" . }}
{{ end }}
`
}

func scaffoldTaxonomyTemplate() string {
	return `{{ define "taxonomyTermsMain" }}
<section class="taxonomy-page">
    <h1>{{ .Title }}</h1>
    <ul>
        {{ range .Terms }}
        <li><a href="{{ .URL }}">{{ .Name }}</a> ({{ .Count }})</li>
        {{ end }}
    </ul>
</section>
{{ end }}

{{ define "taxonomyMain" }}
<section class="taxonomy-page">
    <h1>{{ .Title }}</h1>
    <p><a href="{{ .IndexURL }}">返回列表</a></p>
    <ul>
        {{ range .Posts }}
        <li>
            <span class="date">{{ .Date.Format "2006-01-02" }}</span>
            <a href="{{ .URL }}">{{ .Title }}</a>
        </li>
        {{ end }}
    </ul>
</section>
{{ end }}

{{ define "taxonomyTermsPage" }}
{{ template "base" . }}
{{ end }}

{{ define "taxonomyPage" }}
{{ template "base" . }}
{{ end }}
`
}

func scaffoldHeaderTemplate() string {
	return `{{ define "header" }}
<header class="site-header">
<div class="container">
    <nav class="navbar">
        <div class="logo">
            <a href="/">{{ .Site.Title }}</a>
        </div>
        <ul class="nav-links">
            {{ range .Site.NavbarLinks }}
            <li><a href="{{ .URL }}">{{ .Name }}</a></li>
            {{ end }}
        </ul>
    </nav>
</div>
</header>
{{ end }}

{{ define "headerNested" }}
{{ template "header" . }}
{{ end }}
`
}

func scaffoldFooterTemplate() string {
	return `{{ define "footer" }}
<footer class="site-footer">
<div class="container">
    <p>&copy; {{ now.Format "2006" }} {{ .Site.Author }}. All rights reserved.</p>
    <p>Powered by <a href="https://github.com/mengbin92/gobin">Gobin</a></p>
</div>
</footer>
{{ end }}

{{ define "footerNested" }}
{{ template "footer" . }}
{{ end }}
`
}

func scaffoldSamplePost(siteName string) string {
	date := time.Now().Format("2006-01-02T15:04:05+08:00")
	return fmt.Sprintf(`---
title: "欢迎来到 %s"
date: %s
description: "这是你的第一篇博客文章"
tags: ["欢迎", "Gobin"]
categories: ["随笔"]
draft: false
---

# 欢迎使用 Gobin！

这是一个使用 [Gobin](https://github.com/mengbin92/gobin) 静态博客生成器创建的新博客。

## 开始写作

现在你可以开始在这个目录下撰写你的文章了：

1. 在 _posts 目录中创建新的 Markdown 文件
2. 使用 YAML Front Matter 定义文章元数据
3. 编写你的内容
4. 运行 gobin build 构建站点
5. 运行 gobin serve 本地预览

## 写作格式

文章文件需要以 --- 开头和结尾的 YAML Front Matter：

    ---
    title: "文章标题"
    date: 2026-01-05T10:00:00+08:00
    description: "文章描述"
    tags: ["标签1", "标签2"]
    categories: ["分类"]
    draft: false
    ---

    # 文章内容

    在这里编写你的Markdown内容...

祝你写作愉快！
`, siteName, date)
}

func scaffoldGitignore() string {
	return `# Gobin - Static Blog Generator
# Generated files
public/
resources/

# IDE
.vscode/
.idea/
*.swp
*.swo

# OS
.DS_Store
Thumbs.db

# Logs
*.log

# Temporary files
*.tmp
*.bak
`
}

func scaffoldDefaultCSS() string {
	return `/* Gobin Default Styles */

* {
    margin: 0;
    padding: 0;
    box-sizing: border-box;
}

body {
    font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif;
    line-height: 1.6;
    color: #333;
    background-color: #fff;
}

.container {
    max-width: 1200px;
    margin: 0 auto;
    padding: 0 20px;
}

/* Header */
.site-header {
    background: #fff;
    border-bottom: 1px solid #eee;
    margin-bottom: 2rem;
}

.site-header .container {
    padding: 1rem 20px;
}

.navbar {
    display: flex;
    justify-content: space-between;
    align-items: center;
}

.logo a {
    font-size: 1.5rem;
    font-weight: bold;
    color: #333;
    text-decoration: none;
}

.nav-links {
    display: flex;
    list-style: none;
    gap: 2rem;
}

.nav-links a {
    color: #666;
    text-decoration: none;
    transition: color 0.3s;
}

.nav-links a:hover {
    color: #333;
}

/* Post List */
.post-list {
    margin: 2rem 0;
}

.post-list h1 {
    margin-bottom: 2rem;
    color: #333;
}

.post-list ul {
    list-style: none;
}

.post-list li {
    padding: 1rem 0;
    border-bottom: 1px solid #eee;
}

.post-list .date {
    color: #666;
    font-size: 0.9rem;
    margin-right: 1rem;
}

.post-list a {
    color: #333;
    text-decoration: none;
    font-size: 1.1rem;
}

.post-list a:hover {
    color: #0066cc;
    text-decoration: underline;
}

/* Single Post */
.post {
    max-width: 800px;
    margin: 0 auto;
}

.post h1 {
    margin-bottom: 1rem;
    color: #333;
    line-height: 1.3;
}

.post .meta {
    color: #666;
    margin-bottom: 2rem;
    padding-bottom: 1rem;
    border-bottom: 1px solid #eee;
}

.post .meta .date {
    margin-right: 1rem;
}

.post .meta .tags {
    color: #999;
}

.post .content {
    font-size: 1.1rem;
    line-height: 1.8;
}

.post .content h1,
.post .content h2,
.post .content h3 {
    margin: 2rem 0 1rem;
    color: #333;
}

.post .content p {
    margin-bottom: 1.5rem;
}

.post .content code {
    background: #f4f4f4;
    padding: 0.2rem 0.4rem;
    border-radius: 3px;
    font-family: "Consolas", "Monaco", monospace;
}

.post .content pre {
    background: #f4f4f4;
    padding: 1rem;
    border-radius: 5px;
    overflow-x: auto;
    margin: 1rem 0;
}

.post .content pre code {
    background: none;
    padding: 0;
}

.post .content a {
    color: #0066cc;
    text-decoration: none;
}

.post .content a:hover {
    text-decoration: underline;
}

/* Footer */
.site-footer {
    background: #f9f9f9;
    margin-top: 4rem;
    text-align: center;
    color: #666;
}

.site-footer .container {
    padding: 2rem 20px;
}

.site-footer p {
    margin-bottom: 0.5rem;
}

.site-footer a {
    color: #333;
    text-decoration: none;
}

.site-footer a:hover {
    text-decoration: underline;
}

/* Pagination */
.pagination {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin: 2rem 0;
    padding: 1rem 0;
    border-top: 1px solid #eee;
}

.pagination-info {
    color: #666;
}

.pagination-links a {
    color: #0066cc;
    text-decoration: none;
    padding: 0.5rem 1rem;
    border: 1px solid #0066cc;
    border-radius: 3px;
    transition: all 0.3s;
}

.pagination-links a:hover {
    background: #0066cc;
    color: #fff;
}

/* Tag/Category Pages */
.tag-list,
.category-list {
    list-style: none;
    padding: 0;
}

.tag-list li,
.category-list li {
    display: inline-block;
    margin: 0.5rem;
}

.tag-list a,
.category-list a {
    display: inline-block;
    padding: 0.5rem 1rem;
    background: #f4f4f4;
    color: #333;
    text-decoration: none;
    border-radius: 3px;
    transition: background 0.3s;
}

.tag-list a:hover,
.category-list a:hover {
    background: #e4e4e4;
}

/* Responsive */
@media (max-width: 768px) {
    .container {
        padding: 0 15px;
    }

    .navbar {
        flex-direction: column;
        gap: 1rem;
    }

    .nav-links {
        gap: 1rem;
    }

    .post .content {
        font-size: 1rem;
    }
}
`
}
