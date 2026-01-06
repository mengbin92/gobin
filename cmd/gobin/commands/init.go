package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
)

// InitCmd is the init command
var InitCmd = &cobra.Command{
	Use:   "init [name]",
	Short: "Initialize a new blog site",
	Long: `Initialize a new blog site with the default directory structure.

If name is not provided, uses current directory name.

Example:
  gobin init my-blog
  gobin init`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		var dir string
		if len(args) > 0 {
			dir = args[0]
		} else {
			dir, _ = os.Getwd()
		}

		if err := initializeSite(dir); err != nil {
			fmt.Fprintf(os.Stderr, "Error initializing site: %v\n", err)
			os.Exit(1)
		}
	},
}

// initializeSite creates the directory structure and default files
func initializeSite(dir string) error {
	// Get absolute path
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Get directory name for default title
	dirName := filepath.Base(absDir)

	fmt.Printf("Initializing new blog site in: %s\n", absDir)

	// Create directory structure
	dirs := []string{
		"_posts",
		"assets",
		"assets/css",
		"assets/js",
		"templates",
		"templates/_default",
		"templates/partials",
	}

	for _, d := range dirs {
		path := filepath.Join(absDir, d)
		if err := os.MkdirAll(path, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", path, err)
		}
	}

	// Create default config.yaml
	configContent := getDefaultConfig(dirName)
	if err := writeFile(filepath.Join(absDir, "config.yaml"), configContent); err != nil {
		return fmt.Errorf("failed to create config.yaml: %w", err)
	}

	// Create default templates
	templates := map[string]string{
		"templates/_default/list.html":   getListTemplate(),
		"templates/_default/single.html": getSingleTemplate(),
		"templates/_default/404.html":    get404Template(),
		"templates/partials/header.html": getHeaderTemplate(),
		"templates/partials/footer.html": getFooterTemplate(),
	}

	// Create default CSS
	if err := writeFile(filepath.Join(absDir, "assets/css/style.css"), getDefaultCSS()); err != nil {
		return fmt.Errorf("failed to create default css: %w", err)
	}

	for path, content := range templates {
		if err := writeFile(filepath.Join(absDir, path), content); err != nil {
			return fmt.Errorf("failed to create %s: %w", path, err)
		}
	}

	// Create a sample post
	samplePost := getSamplePost(dirName)
	postDate := time.Now().Format("2006-01-02")
	postName := filepath.Join(absDir, "_posts", postDate+"-welcome.md")
	if err := writeFile(postName, samplePost); err != nil {
		return fmt.Errorf("failed to create sample post: %w", err)
	}

	// Create .gitignore
	gitignore := getGitignore()
	if err := writeFile(filepath.Join(absDir, ".gitignore"), gitignore); err != nil {
		return fmt.Errorf("failed to create .gitignore: %w", err)
	}

	fmt.Println("Site initialized successfully!")
	fmt.Println("\nNext steps:")
	fmt.Println("  cd " + dirName)
	fmt.Println("  gobin build")
	fmt.Println("  gobin serve")

	return nil
}

func writeFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0644)
}

func getDefaultConfig(name string) string {
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
`, name)
}

func getBaseTemplate() string {
	return `<!DOCTYPE html>
<html lang="{{ .Site.LanguageCode }}">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>{{ if .Title }}{{ .Title }} | {{ .Site.Title }}{{ else }}{{ .Site.Title }}{{ end }}</title>
    <meta name="description" content="{{ .Site.Description }}">
    <link rel="stylesheet" href="/assets/css/style.css">
</head>
<body>
    {{ template "header" . }}

    <main class="container">
        {{ template "content" . }}
    </main>

    {{ template "footer" . }}
</body>
</html>
`
}

func getListTemplate() string {
	return `{{ define "listPage" }}
<!DOCTYPE html>
<html lang="{{ .Site.LanguageCode }}">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>{{ if .Title }}{{ .Title }} | {{ .Site.Title }}{{ else }}{{ .Site.Title }}{{ end }}</title>
    <meta name="description" content="{{ .Site.Description }}">
    <link rel="stylesheet" href="/css/style.css">
</head>
<body>
    {{ template "header" . }}

    <main class="container">
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
        </div>
    </main>

    {{ template "footer" . }}
</body>
</html>
{{ end }}
`
}

func getSingleTemplate() string {
	return `{{ define "singlePage" }}
<!DOCTYPE html>
<html lang="{{ .Site.LanguageCode }}">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>{{ if .Title }}{{ .Title }} | {{ .Site.Title }}{{ else }}{{ .Site.Title }}{{ end }}</title>
    <meta name="description" content="{{ if .Post.Description }}{{ .Post.Description }}{{ else }}{{ .Site.Description }}{{ end }}">
    <link rel="stylesheet" href="/css/style.css">
</head>
<body>
    {{ template "header" . }}

    <main class="container">
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
    </main>

    {{ template "footer" . }}
</body>
</html>
{{ end }}
`
}

func get404Template() string {
	return `{{ define "404Page" }}
{{ template "header" . }}
<div class="error-page">
    <h1>404 - Page Not Found</h1>
    <p>The page you're looking for doesn't exist.</p>
    <a href="/">Go back home</a>
</div>
{{ template "footer" . }}
{{ end }}
`
}

func getHeaderTemplate() string {
	return `{{ define "header" }}
<header class="site-header">
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
</header>
{{ end }}
`
}

func getFooterTemplate() string {
	return `{{ define "footer" }}
<footer class="site-footer">
    <p>&copy; {{ now.Format "2006" }} {{ .Site.Author }}. All rights reserved.</p>
    <p>Powered by <a href="https://github.com/mengbin92/gobin">Gobin</a></p>
</footer>
{{ end }}
`
}

func getSamplePost(siteName string) string {
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

func getGitignore() string {
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

func getDefaultCSS() string {
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
    padding: 1rem 0;
    margin-bottom: 2rem;
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
    padding: 2rem 0;
    margin-top: 4rem;
    text-align: center;
    color: #666;
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
