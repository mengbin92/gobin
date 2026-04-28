# Gobin v1.0.0 发布说明

## 🎉 正式发布 - 2026-01-07

**Gobin v1.0.0 正式发布！** 这是一个功能完整、经过充分测试的高性能静态博客生成器。

---

## 快速开始

### 安装

```bash
go install github.com/mengbin92/gobin/cmd/gobin@latest
```

### 创建你的第一个站点

```bash
mkdir my-blog && cd my-blog
gobin init
gobin build
gobin serve
```

访问 http://localhost:8080 查看你的博客！

---

## 🌟 核心特性

### 1. 极速性能
- ⚡ 比 Jekyll 快 **10-100 倍**
- 100 篇文章构建时间 **< 0.2 秒**
- 内存占用 **< 150MB** / 1000篇文章

### 2. Jekyll 迁移友好
- ✅ 支持常见 Jekyll 博客内容结构
- ✅ 迁移现有 Markdown 内容
- ✅ 支持文章 permalink 配置（SEO 友好）

### 3. 功能完备
- Markdown 渲染（完整语法支持）
- 标签 + 分类系统
- 分页功能
- RSS/Atom Feed
- Sitemap.xml
- 搜索索引
- SEO 优化

### 4. 灵活配置
- YAML 配置文件
- 自定义主题
- 多平台部署
- 评论系统集成
- 分析工具集成

---

## 📦 安装选项

### 方式一：从源码安装（推荐）

```bash
git clone https://github.com/mengbin92/gobin.git
cd gobin
go build -o gobin ./cmd/gobin
sudo mv gobin /usr/local/bin/
```

### 方式二：使用 go install

```bash
go install github.com/mengbin92/gobin/cmd/gobin@latest
```

### 方式三：下载预编译二进制

前往 GitHub Releases 页面下载适合你平台的二进制文件。

---

## 🛠️ 命令参考

| 命令 | 说明 |
|------|------|
| `gobin init [name]` | 初始化新站点 |
| `gobin build` | 构建静态站点 |
| `gobin serve` | 启动开发服务器 |
| `gobin new post [title]` | 创建新文章 |
| `gobin new page [title]` | 创建新页面（可用）|
| `gobin version` | 显示版本信息 |
| `gobin check` | 检查配置和文章（可用）|

---

## 📖 文档

### 项目文档
- [用户指南](README.md) - 完整的使用说明
- [示例站点](example-site/) - 可运行的完整示例
- [技术方案](static-blog-technical-proposal.md) - 技术架构说明
- [测试报告](docs/test-coverage-report.md) - 测试覆盖详情

### 部署文档
- [GitHub Pages 部署](README.md#github-pages-部署)
- [Vercel 部署](README.md#vercel-部署)
- [Netlify 部署](README.md#netlify-部署)

---

## 🎯 从 Jekyll 迁移

从 Jekyll 迁移到 Gobin 非常简单：

```bash
# 1. 备份你的 Jekyll 站点
git clone your-jekyll-repo.git backup

# 2. 创建新的 Gobin 站点
gobin init my-new-blog
cd my-new-blog

# 3. 复制内容
cp -r ../backup/_posts ./_posts
cp -r ../backup/assets ./assets

# 4. 转换配置
touch config.yaml
# 从 _config.yml 复制配置项

# 5. 构建和测试
gobin build
gobin serve
```

完整迁移指南请查看 [迁移文档](README.md#从-jekyll-迁移)。

---

## 🌐 部署平台

### GitHub Pages
使用 GitHub Actions 自动部署：

```yaml
# .github/workflows/deploy.yml
name: Deploy to GitHub Pages

on:
  push:
    branches: [ main ]

jobs:
  deploy:
    runs-on: ubuntu-latest
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

### Vercel / Netlify
直接连接 Git 仓库，配置：
- **构建命令**: `gobin build --minify`（当前为保守 HTML/CSS 压缩）
- **输出目录**: `public`

---

## 📊 性能对比

| 博客规模 | Jekyll | Gobin | 速度提升 |
|---------|--------|-------|---------|
| 100 篇文章 | 15-30 秒 | 0.2 秒 | 75x-150x |
| 1000 篇文章 | 2-5 分钟 | 2-3 秒 | 40x-100x |
| 内存占用 | 高 | ~150MB/1000篇 | 显著降低 |

---

## 🧪 质量保证

### 测试覆盖
- **Config 模块**: 100% 覆盖率
- **Parser 模块**: 84.6% 覆盖率
- **Generator 模块**: 核心功能全覆盖
- **总计**: 26 个单元测试，3 个集成测试

### 代码质量
- 遵循 Go 最佳实践
- 完善的错误处理
- 完整的文档注释
- 性能基准测试

---

## 🎓 示例站点

项目包含一个完整的示例站点：

```bash
cd example-site
gobin build
gobin serve
```

示例站点展示了：
- 文章列表和详情页
- 标签和分类页面
- 响应式设计
- 暗色模式支持
- 完整的配置示例
- SEO 优化示例

---

## 🆘 支持和社区

### 获取帮助
如遇到问题：
1. 查看 [FAQ](README.md#常见问题)
2. 提交 [GitHub Issue](https://github.com/mengbin92/gobin/issues)
3. 在 [Discussions](https://github.com/mengbin92/gobin/discussions) 中提问

### 报告问题
- 提交详细的错误描述
- 包含复现步骤
- 提供相关配置文件
- 附带错误日志

### 参与贡献
欢迎贡献代码、文档或建议！

查看 [CONTRIBUTING.md](CONTRIBUTING.md) 了解如何贡献。

---

## 🔄 版本历史

### v1.0.0 (2026-01-07)
- ✅ 核心功能
- ✅ 进阶功能
- ✅ 功能完善
- ✅ 测试与优化

**状态**: 稳定版本，可用于生产环境

---

## 🔮 路线图

### 短期 (v1.x)
- [ ] 增量构建优化
- [ ] 更多内置主题
- [ ] 图片自动优化
- [ ] 短代码系统

### 中期 (v2.0)
- [ ] 内容调度
- [ ] WebP 图片生成
- [ ] 内容版本管理
- [ ] Webhooks 集成

### 长期
- [ ] 插件系统
- [ ] 图形化管理界面
- [ ] 云同步服务

---

## 📝 许可证

MIT 许可证 - 详见 [LICENSE](LICENSE) 文件

---

## 🙏 致谢

感谢所有为 Gobin 做出贡献的开发者、测试者和社区成员！

特别感谢：
- 所有提交 Issue 和 PR 的贡献者
- 提供反馈的早期用户
- 技术文档编写者

---

## ⚖️ 与 Jekyll 的对比

### 优势
1. **性能**: 10-100 倍速度提升
2. **部署**: 单二进制，零依赖
3. **内存**: 更低内存占用
4. **构建**: 快速生成静态页面和站点产物
5. **兼容性**: 支持常见 Jekyll 内容结构

### 注意事项
- 模板语法不同（Go template vs Liquid）
- 插件生态较小（但正在成长）
- 部分高级功能需要手动配置

---

## 🎈 最后的话

Gobin 的目标是让静态博客生成再次变得简单、快速、愉悦。

我们相信：
- 简单胜过复杂
- 快速胜过缓慢
- 现代胜过繁琐

**欢迎加入 Gobin 社区！** 🎉

---

**开始你的 Gobin 之旅吧！**

```bash
go install github.com/mengbin92/gobin/cmd/gobin@latest
gobin init my-blog
cd my-blog
gobin serve
```

访问 http://localhost:8080，开始写作！✍️

---

*发布日期: 2026-01-07*
*版本: v1.0.0*
*作者: Mengbin*
*GitHub: [mengbin92/gobin](https://github.com/mengbin92/gobin)*
