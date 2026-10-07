# Gobin v1.9 更新日志

## v1.9.0 - 2026-10-07

Gobin v1.9.0 引入**多语言站点支持**：`languages:` 声明多语言，每种语言生成独立的页面树、分页、taxonomy、feed、sitemap、搜索索引和 404，输出在 `/<lang>/` 前缀下；默认语言（顶层配置）仍在站点根，路径与单语言完全一致。模板新增 `T` 文案翻译函数、`.Languages` 语言切换数据、`translationKey` 跨语言文章互链。增量构建按语言独立清单，跨语言翻译变化精确触发重渲染；`serve --watch` 监听并按语言路由所有语言的内容目录。无 `languages:` 的站点输出与 v1.8.5 字节级一致。详见 [v1.9.0 发布说明](./RELEASE-NOTES-v1.9.0.md)。

---

## 新增功能

### 多语言配置与内容组织（v1.9.0）

- `config.yaml` 新增 `languages:` map：每语言可覆盖 `languageCode`/`title`/`description`/`contentDir`/`pageDir`/`params`/`strings`，默认值分别为语言 key、继承顶层、`<contentDir>/<key>`、`<pageDir>/<key>`。
- 顶层配置即默认语言（站点根）；`languages:` 只声明额外语言。
- 新增顶层 `strings:`（默认语言 UI 文案表）。
- 语言 key 校验：单 URL 段、不与保留根段（paginatePath/tags/categories/staticDirs/publishDir）冲突、目录不重复不重叠。

### 按语言生成

- 每语言独立：文章页、列表分页、tags/categories、`index.xml`/`index.atom`、`sitemap.xml`、`search-index.json`(+`-min`)、`404.html`、静态资源副本、增量清单（`publishDir/<lang>/.gobin-build.json`）。
- `<html lang>` 自动使用语言 `languageCode`；canonical/feed/sitemap/search URL 自动带 `/<lang>/` 前缀。
- prev/next、列表、taxonomy 严格按语言隔离；`robots.txt` 与 aliases 只在默认语言生成。

### 模板

- funcMap 新增 `T(key)`：语言 strings → 顶层 strings → key 原样。
- `BasePageData` 新增 `Lang` 与 `Languages`（`LanguageLink{Lang, Name, URL, Active}`）。
- `parser.Post` 新增 `translationKey` front matter 与 `Translations` 互链（按语言排序、排除自身、草稿过滤）。
- 仓库模板/主题/脚手架内部链接全部改经 `url` 函数（单语言恒等）；自定义模板需同样迁移。

### 构建 / 检查 / serve

- `gobin build`：默认语言先构建，各语言按 key 排序；clean 构建顺带清除已删除语言的残留子树。
- 增量构建：翻译链接折入文章 list 指纹，单页跳过收紧为 SourceHash+ListHash 双比较（单语言行为不变）。
- `gobin check`：逐语言碰撞检查，路径带语言前缀。
- `gobin serve --watch`：监听所有语言内容目录；内容缓存按路径路由回语言桶；语言页面按所属语言 pageDir 重解析。

## 兼容性

- 公开 API 100% 向后兼容（全部新增）；无 `languages:` 站点输出字节级不变（golden 原样通过）。
- manifest schema 不变（version 2）。

## 已知限制 / Deferred

- 非默认语言 aliases 禁用；assets/图片变体按语言复制；无 sitemap index；独立页无翻译互链；不支持 `post.en.md` 后缀、`:lang` permalink token、`i18n/` 目录文件。
