# Gobin 示例站点

这是一个使用 Gobin 构建的示例博客网站，展示了 Gobin 的所有核心功能。

## 特性演示

### 1. Jekyll 兼容性
- 使用 Markdown 编写文章
- YAML Front Matter 格式
- 文章存放在 `_posts/` 目录
- 静态资源在 `assets/` 目录

### 2. 强大的内容管理
- 标签 (Tags) 系统
- 分类 (Categories) 系统
- 文章摘要生成
- 阅读时间计算

### 3. 主题系统
- 可自定义的模板
- 响应式设计
- 支持主题目录

### 4. 现代功能
- SEO 优化 (OpenGraph, Twitter Card)
- Sitemap.xml
- RSS/Atom Feeds
- 搜索索引生成

### 5. 性能优化
- 超快构建速度
- 低内存占用
- 静态资源复制优化

## 如何使用

```bash
# 构建站点
gobin build

# 启动开发服务器
gobin serve
```

## 示例文章

查看 `_posts/` 目录中的示例文章，了解如何：
- 使用 Front Matter
- 添加标签和分类
- 插入代码块
- 添加图片

## 自定义

- 修改 `config.yaml` 配置站点设置
- 编辑 `templates/` 中的模板自定义外观
- 修改 `assets/css/style.css` 自定义样式
