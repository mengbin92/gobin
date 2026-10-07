# Gobin v1.9.0 发布说明

## 发布日期 - 2026-10-07

Gobin v1.9.0 引入**多语言站点支持**，补齐 README 开发计划第三阶段的最后一项。站点在 `config.yaml` 声明 `languages:` 后，每种语言拥有独立的内容目录、页面树、feed、sitemap、搜索索引、分页和 taxonomy，输出在 `/<lang>/` 前缀下；默认语言仍在站点根，**无 `languages:` 配置的站点输出与 v1.8.5 字节级一致**（全部既有 golden 测试原样通过）。

## 新增功能

### 多语言配置

```yaml
title: My Blog          # 顶层配置 = 默认语言（站点根，路径不变）
languageCode: en
strings:
  readMore: Read more

languages:
  zh:
    languageCode: zh-CN  # 默认 = 语言 key
    title: 我的博客       # 默认继承顶层
    contentDir: _posts/zh # 默认 <contentDir>/<key>
    pageDir: pages/zh     # 默认 <pageDir>/<key>
    params: {}            # 浅合并覆盖顶层 params
    strings:              # 合并覆盖顶层 strings
      readMore: 阅读更多
```

语言 key 须为单 URL 段（`^[a-z0-9][a-z0-9-]*$`）且不与 `paginatePath`/`tags`/`categories`/静态目录/`publishDir` 冲突；校验在配置加载期完成。

### 按语言的完整站点生成

每种语言独立生成：文章页、列表分页、tags/categories、`index.xml`/`index.atom`、`sitemap.xml`、`search-index.json`（含 `-min`）、`404.html`、静态资源副本。`<html lang="...">` 自动使用该语言的 `languageCode`；canonical/feed/sitemap 绝对 URL 自动带 `/<lang>/` 前缀。prev/next、列表页、taxonomy 严格按语言隔离。

### 模板能力

- **`T` 函数**：`{{ T "readMore" }}` 按当前语言查 `strings`（语言 → 顶层 → key 原样）。
- **`.Languages` / `.Lang`**：每个页面数据携带语言列表（含 Active 标记）与当前语言，用于语言切换器。
- **翻译互链**：不同语言文章 front matter 写相同 `translationKey`，模板经 `.Post.Translations` 拿到互链（标题、语言名、带前缀 URL），可手写 hreflang。
- **内部链接约定**：仓库模板/主题/`gobin init` 脚手架的内部链接全部改经 `url` 辅助函数（单语言下恒等，行为不变）。自定义模板直接写 `{{ .URL }}` / `href="/tags/..."` 的，在非默认语言下会链接到默认语言页面，需迁移为 `{{ url .URL }}`。

### 构建与开发体验

- `gobin build`：默认语言先构建（`--clean` 顺带清除已删除语言的残留子树），各语言按 key 排序依次构建。
- **增量构建**：每语言独立 `.gobin-build.json`；改中文文章不失效英文产物；改中文文章 slug/title 会让互链它的其他语言文章自动重渲染（翻译链接折入文章 list 指纹，单页跳过条件同步收紧）。
- `gobin check`：逐语言 permalink 碰撞检查（跨语言同 slug 不算碰撞），碰撞路径带语言前缀。
- `gobin serve --watch`：监听所有语言内容目录（含自定义外部目录），内容缓存按路径路由回语言桶做增量重解析；修复了 watch 重建丢失语言内容的问题。

## 库 API 变化

- `config.Config` 新增 `Languages` / `Strings` / `ActiveLanguage` / `RootBaseURL` / `RootLanguageCode` 字段与 `IsMultilingual` / `LanguageNames` / `ResolveLanguage` / `DeriveForLanguage` 方法；新增 `config.LanguageConfig`。
- `parser.Post` 新增 `TranslationKey` / `Lang` / `Translations`；`parser.Page` 新增 `Lang`；新增 `parser.TranslationLink`。
- `parser` 新增 `ParsePostsWithOptionsConcurrentExclude` / `ParsePagesWithOptionsConcurrentExclude`（现有签名不变，内部委托）。
- `generator` 新增 `GenerateMultilingualWithOptions` / `DryRunMultilingual` / `LanguageContent`；`BasePageData` 新增 `Lang` / `Languages`（`LanguageLink`）；funcMap 新增 `T`。
- 全部为新增，无破坏性变更。

## 兼容性

- 无 `languages:` 的站点：配置、CLI、模板数据、公开 Go API、构建产物均与 v1.8.5 一致（字节级，golden 测试原样通过）。
- 增量清单 schema 不变（`buildManifestVersion` 保持 2）；单语言→多语言切换经环境哈希自动触发全量重建。
- 单页增量跳过条件收紧为 SourceHash + ListHash 双比较：单语言下两者同变，行为不变。

## 当前限制

- aliases 重定向页只在默认语言生成（非默认语言的 alias 目标缺语言前缀，会指错，故禁用）。
- 静态资源与图片变体按语言复制（`public/zh/assets/...`），保证各语言子树自洽。
- `robots.txt` 只在站点根；无跨语言 sitemap index；独立页无翻译互链；不支持 `post.en.md` 文件名后缀、`:lang` permalink token、`i18n/` 目录文案文件。

## 文档

- 用户指南：`docs/guides/multilingual.md`
- 设计文档：`docs/design/2026-10-07-multilingual-design.md`

## 验证

发布前执行：

```bash
go test ./...
go test -race ./internal/parser/... ./internal/generator/... ./cmd/gobin/commands/...
go vet ./...
gofmt -l internal/ cmd/
```

新增测试：config 派生/校验、parser 排除与 translationKey、linkTranslations、artifact 门控、增量跨语言失效、双语端到端、CLI 双语 build/check、serve 缓存分桶，以及新 golden 站点 `multilingual_site`（`GOBIN_GOLDEN_UPDATE=multilingual_site` 可再生成夹具）。
