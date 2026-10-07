# 多语言支持设计（v1.9.0）

- 日期：2026-10-07
- 状态：已实现
- 用户指南：`docs/guides/multilingual.md`

## 1. 背景与目标

README 开发计划第三阶段只剩"多语言支持"未完成。目标：站点可声明多种语言，每种语言生成独立页面树/feed/sitemap/搜索/分页/taxonomy，模板可感知语言并翻译 UI 文案，文章可跨语言互链；无 `languages:` 配置的站点输出字节级不变。

## 2. 核心架构：派生配置 + 每语言一次生成

**每种语言用"派生配置 + 语言子目录输出"跑一遍现有单语言管线。**

选型依据（已验证）：

- `prepareGenerationPlan(posts, pages, cfg, outputDir, ...)` 完全由 cfg + outputDir 参数化，其下所有 URL 规则、分页、taxonomy、feed、sitemap、manifest 都从这两个输入派生。
- 所有绝对 URL 走 `joinURL(cfg.BaseURL, path)`：派生配置把 `BaseURL` 扩展为 `<base>/<lang>`，canonical/feed/sitemap 自动带语言前缀。
- 所有输出路径相对 outputDir：派生运行用 `publishDir/<lang>`，全部产物（含每语言 `.gobin-build.json`）自动落到语言前缀下。
- `<html lang="{{ .Site.LanguageCode }}">` 已在模板中，派生配置覆盖 `LanguageCode` 即正确。
- `loadTemplates(cfg)` 的 funcMap 闭包捕获 cfg：`T` 函数每语言天然隔离。

结论：**posts.go / pages.go / taxonomy.go / feed.go / sitemap.go / search.go 零改动**；多语言逻辑集中在新的 `multilingual.go`（编排）、config（schema/派生）、CLI（内容解析路由）。

被否决的替代方案：

- **URL 加前缀、单一 outputDir**：`post.URL = "/zh/slug/"` 会让 feed/sitemap 的 `joinURL(BaseURL+/zh, /zh/slug/)` 双重前缀，且 feed/sitemap/404/manifest 的文件名在根目录互相覆盖，破坏管线复用。
- **文件名后缀内容组织**（`post.en.md`）：与 slug/日期前缀解析耦合，对 Jekyll 迁移用户陌生。

## 3. 配置模型

顶层配置**即**默认语言（根路径、行为与单语言一致）；`languages:` 只声明额外语言。

```yaml
languages:
  zh:
    languageCode: zh-CN   # 默认 = key
    title: ...            # 默认继承顶层
    description: ...
    contentDir: ...       # 默认 <contentDir>/<key>
    pageDir: ...          # 默认 <pageDir>/<key>
    params: {}            # 浅合并覆盖顶层
    strings: {}           # 合并覆盖顶层，供 T 使用
```

关键决策：

- **惰性默认值**：`Normalize` 不触碰 `languages`，默认值在 `ResolveLanguage` 解析时填充——增量构建 env hash 只反映用户实际写下的配置。
- **派生配置**（`DeriveForLanguage`）：浅克隆后应用覆盖、合并 Params/Strings、扩展 BaseURL、设置 `ActiveLanguage`，并保存 `RootBaseURL`/`RootLanguageCode` 供语言切换器指回默认语言。后三个字段 `yaml:"-" json:"-"`，不进 env hash。
- **校验**：key 必须匹配 `^[a-z0-9][a-z0-9-]*$` 且不与保留根段（paginatePath/tags/categories/staticDirs/publishDir）冲突；语言目录不得等于默认目录/publishDir，语言间不得重复。

## 4. 内容解析：目录约定 + 排除

约定优于配置：默认 `<contentDir>/<lang>/`、`<pageDir>/<lang>/`，可按语言覆盖为任意外部目录。

**已验证的坑**：`collectMarkdownFiles` 递归遍历，`_posts/zh/*.md` 会泄漏进默认语言。解决：parser 新增 `collectMarkdownFilesExcluding`（`filepath.SkipDir` 剪枝）与 `ParsePosts/PagesWithOptionsConcurrentExclude` 导出包装；现有签名委托传 `nil`，零破坏。CLI 的 `parseSiteContent` 只排除**嵌套**在默认目录内的语言目录（外部目录本就不会被默认语言扫到）。

## 5. 生成编排

`GenerateMultilingualWithOptions`（`internal/generator/multilingual.go`）：

1. `buildLanguageRuns`：run[0] = 默认语言（原 cfg、原 outputDir）；之后按**排序后**的语言 key 每语言一个派生 run。默认语言先跑——`cleanOutputDir` 清空整个 publishDir，顺带移除已删除语言的残留子树。
2. `linkTranslations`：按 `translationKey` 分组跨语言互链，回填 `Post.Lang`/`Post.Translations`（URL 用 `siteURLPath(语言BaseURL, postURL)` 预算，含前缀）。草稿由 `isVisiblePost` 统一过滤。
3. 逐 run 执行现有 `prepareGenerationPlan` + `ExecuteResult`（v1 串行；各 run 目录不相交，未来可并行），统计合并。

门控（`artifacts.go`）：`robots` 仅默认语言（根级文件）；`aliases` 非默认语言禁用——alias 目标由无前缀的 `post.URL` 派生，会指错（最意外的限制，文档显著标注）。

页面数据：`BasePageData` 新增 `Lang`/`Languages []LanguageLink`，在 6 处构造点由 `languageLinksFor(cfg)` 填充；单语言时为零值，既有模板无感知。

## 6. 模板与内部链接

- funcMap 新增 `T(key)`：读 `cfg.Strings`（派生配置已合并），未命中回退 key 本身。
- **内部链接前缀问题**：既有模板大量硬编码根相对链接（`{{ .URL }}`、`/tags/...`、`/page/2/`）。修复方式是把**计算生成的**内部链接全部改经既有的 `url` 函数（`siteURLPath` 按 baseURL 路径补前缀）。单语言 + 无路径 baseURL 下 `url` 是恒等转换——golden 测试字节级不变。用户自定义链接（navbarLinks）保持原样。
- 仓库模板、`themes/example`、`themes/official-website`、`gobin init` 脚手架、example-site 已全部改写。
- 搜索索引文档 URL 同样改经 `siteURLPath`（feed/sitemap 本就走 `joinURL`，自动正确）。

## 7. 增量构建

**每语言一个 manifest**：每 run 的 `writeBuildManifest(run.OutputDir, ...)` 把 `.gobin-build.json` 写进各自输出子树；`buildManifestVersion` 保持 2，无 schema 变更。单语言→多语言切换经 config 部分的 env hash 自动触发全量重建。

**跨语言失效**（唯一的跨语言依赖）：zh 文章改 slug/title 会让互链它的 en 文章页面过期，但 en 文章的源文件字节没变。解决：

1. `computePostCategoryHashes` 在 `Translations` 非空时将其稳定序列化折入 ListHash（单语言为空，现有哈希不变）；
2. `applyIncrementalSkips` 的单页跳过条件从"SourceHash 相等"收紧为"SourceHash **且** ListHash 相等"。单语言下 SourceHash 相等蕴含 ListHash 相等（解析确定性），行为不变；多语言下翻译变化精确触发重渲染。

## 8. serve --watch

- `watchPaths` 追加各语言解析后的内容目录（嵌套的已被递归监听覆盖，靠去重吸收）。
- `classifyChange` 识别外部语言目录的 markdown 为 content/page 类变更。
- **contentCache 语言分桶**（关键修复）：原实现只缓存默认语言内容，watch 重建会丢失所有语言文章（clean 重建后语言子树整个消失）。现在所有语言共用一个 path 键控缓存，`assemble` 按最长前缀匹配把路径路由回语言桶；语言页面的增量重解析用其**所属语言的 pageDir** 作 baseDir，保证 URL 推导正确。

## 9. 已接受的权衡

- **静态资源与图片变体按语言复制**：`assetURL` 从 BaseURL 派生基路径，共享根资源会产生断链；复制保证各子树自洽。去重（如资源留在根、各语言链接指回）列入后续候选。
- 模板每语言各解析一次（funcMap 闭包需要）——构建耗时随语言数线性增长，可接受。

## 10. 明确 deferred

文件名后缀内容（`post.en.md`）；`:lang` permalink token/按语言 permalink；`i18n/` 目录文案文件；跨语言共享 assets/图片变体；sitemap index + robots `Sitemap:` 行；跨语言 aliases；自动 hreflang（可用 `.Post.Translations` 手写）；独立页翻译互链；按语言 theme/staticDir；`defaultContentLanguage`；多语言并行构建。

## 11. 测试策略

- 既有 golden（`default_site`、`official_theme`）**原样通过** = 字节级兼容闸门。
- 新 golden `multilingual_site`（en + zh 全产物快照）；harness 增加 `GOBIN_GOLDEN_UPDATE` 更新模式，`normalizeGoldenContent` 改按 basename 匹配以覆盖 `<lang>/` 下的同名产物。
- 单测：config 派生/合并/校验、parser 排除与 `translationKey`、linkTranslations、languageLinksFor、artifact 门控、哈希耦合、DryRun 分语言。
- 端到端：双语构建产物树、`lang` 属性、`T` 译文、互链、prev/next 隔离、canonical 前缀；增量（二次构建全跳过；改 zh slug → en 互链文章重渲染）。
- CLI：双语 build/check；serve 的 watchPaths/classifyChange/contentCache 分桶与语言文件增量重解析。
