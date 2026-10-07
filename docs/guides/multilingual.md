# 多语言站点（multilingual）使用指南

> Gobin v1.9.0 起支持多语言站点：在 `config.yaml` 里声明 `languages:`，每种语言拥有独立的内容目录、页面树、feed、sitemap、搜索索引、分页和 taxonomy，输出在 `/<lang>/` 前缀下。默认语言（顶层配置）仍在站点根，路径与单语言时完全一致。

## 1. 为什么需要

一个博客想用中英文同时发布时，单语言模型只有两个选择：混在一个站点里（列表页、feed、搜索全都串味），或者跑两个站点（模板、配置、资源全部重复维护）。多语言模式让一套配置/模板/静态资源服务多种语言：内容按语言分目录，构建一次产出所有语言的完整站点。

## 2. 快速开始

```yaml
# config.yaml
title: My Blog
languageCode: en
baseURL: https://example.com

# 默认语言（英文）的 UI 文案（可选）
strings:
  readMore: Read more

languages:
  zh:
    languageCode: zh-CN        # 可选，默认就是语言 key
    title: 我的博客             # 可选，默认继承顶层 title
    strings:                   # 可选，合并覆盖顶层 strings
      readMore: 阅读更多
```

内容目录按约定放在默认目录的语言子目录下：

```
_posts/           # 默认语言（英文）文章
_posts/zh/        # 中文文章
pages/            # 默认语言独立页
pages/zh/         # 中文独立页
```

`gobin build` 后：

```
public/           # 默认语言完整站点（与单语言输出一致）
public/zh/        # 中文完整站点：index.html、文章页、分页、tags/、
                  # categories/、index.xml、index.atom、sitemap.xml、
                  # search-index.json、404.html、静态资源副本
```

每种语言独立分页、独立 taxonomy、独立 feed/sitemap/搜索索引；文章页 `<html lang="...">` 自动使用该语言的 `languageCode`。

## 3. 配置参考

`languages.<key>` 支持的字段：

| 字段 | 默认值 | 说明 |
|------|--------|------|
| `languageCode` | 语言 key | `<html lang>` 与 RSS `<language>` 使用的代码 |
| `title` | 继承顶层 | 该语言的站点标题 |
| `description` | 继承顶层 | 该语言的站点描述 |
| `contentDir` | `<contentDir>/<key>` | 该语言的文章目录 |
| `pageDir` | `<pageDir>/<key>` | 该语言的独立页目录 |
| `params` | — | 浅合并覆盖顶层 `params` |
| `strings` | — | 合并覆盖顶层 `strings`，供 `T` 函数使用 |

语言 key 的约束（会成为 URL 前缀）：

- 只能是小写字母/数字/连字符（`^[a-z0-9][a-z0-9-]*$`）
- 不能与保留根段冲突：`paginatePath`（默认 `page`）、`tags`、`categories`、`staticDir`/`staticDirs` 首段、`publishDir`
- 各语言的 `contentDir`/`pageDir` 不能相同，不能等于默认目录，也不能等于 `publishDir`

## 4. 模板

### 4.1 UI 文案翻译：`T` 函数

```html
<a href="{{ url .URL }}">{{ T "readMore" }}</a>
```

查找顺序：当前语言 `strings` → 顶层 `strings` → key 本身（原样输出）。

### 4.2 语言切换器：`.Languages`

每个页面数据都带 `Languages` 列表（默认语言在前）和当前语言 `Lang`：

```html
<nav class="lang-switch">
  {{ range .Languages }}
    {{ if .Active }}
      <span class="active">{{ .Name }}</span>
    {{ else }}
      <a href="{{ .URL }}">{{ .Name }}</a>
    {{ end }}
  {{ end }}
</nav>
```

### 4.3 同一文章的翻译互链：`translationKey` + `.Post.Translations`

在不同语言的文章 front matter 里写相同的 `translationKey`：

```yaml
# _posts/2026-03-20-hello.md
title: Hello
translationKey: hello-2026
---
```

```yaml
# _posts/zh/2026-03-20-hello.md
title: 你好
translationKey: hello-2026
---
```

模板里渲染互链（也可用于手写 hreflang）：

```html
{{ if .Post.Translations }}
<ul class="translations">
  {{ range .Post.Translations }}
  <li><a href="{{ .URL }}" hreflang="{{ .Name }}">{{ .Title }}</a></li>
  {{ end }}
</ul>
{{ end }}
```

`Translations` 按语言排序、不包含文章自身；URL 已带语言前缀（如 `/zh/hello/`）。草稿不参与互链（`--drafts` 构建时参与）。

### 4.4 内部链接必须用 `url` 辅助函数

非默认语言页面的所有根相对链接都需要 `/<lang>/` 前缀。`url` 函数会根据当前语言的 baseURL 自动补前缀：

```html
<a href="{{ url .URL }}">...</a>                        <!-- 文章链接 -->
<a href="{{ url (printf "/tags/%s/" (urlize .)) }}">...</a>  <!-- 标签链接 -->
<a href="{{ url "/" }}">首页</a>                          <!-- 站点首页 -->
```

仓库自带模板、主题与 `gobin init` 脚手架已全部改用 `url`。**自定义模板若直接写 `{{ .URL }}` 或 `href="/tags/..."`，在非默认语言下会链接到默认语言的页面**，迁移时逐处替换即可（单语言站点下 `url` 是恒等转换，行为不变）。

## 5. 构建、检查与开发服务器

- `gobin build`：依次构建默认语言与各语言（语言按 key 排序）。`--clean`（默认）会先清空 `publishDir`，顺带移除已删除语言的残留子树。
- `gobin build --incremental --clean=false`：每种语言有独立的增量清单（`publishDir/.gobin-build.json` 与 `publishDir/<lang>/.gobin-build.json`）。改中文文章不会让英文产物失效；改中文文章的 slug/title 会让互链它的英文文章自动重渲染。
- `gobin check`：逐语言做 permalink 碰撞检查，碰撞路径带语言前缀（如 `zh/hello/index.html`）。同一 slug 出现在不同语言**不算**碰撞。
- `gobin serve --watch`：监听所有语言的内容目录（包括自定义的外部目录），改动按语言路由到缓存做增量重解析；配置/模板变更仍触发全量重建。

## 6. 当前限制

- **aliases（重定向页）只在默认语言生成**。alias 目标由 `post.URL` 派生、路径是站点绝对的，在 `/<lang>/` 子树下会指错，因此非默认语言的 alias 被禁用（构建不报错，直接不生成）。
- **静态资源与图片变体按语言复制**（`public/zh/assets/...`），保证每个语言子树自洽；产物体积与构建时间随语言数线性增长。
- `robots.txt` 只在站点根生成；无跨语言 sitemap index。
- 独立页（pages）没有翻译互链；`translationKey` 只对文章生效。
- 不支持文件名后缀式多语言（`post.en.md`）、`:lang` permalink token、按语言独立 permalink 规则、`i18n/` 目录文案文件。
- 正文里手写的绝对路径链接（如 `/assets/...`）不会按语言改写（与 baseURL 带子路径时的既有行为一致）。
- 模板导航（`navbarLinks` 等用户自定义链接）原样输出，不自动加语言前缀。

## 7. 排错

| 症状 | 可能原因 | 处理 |
|------|----------|------|
| 默认语言列表页混进了其他语言的文章 | 语言目录不是 `<contentDir>/<lang>/` 且未在 `languages` 声明 | 检查 `languages.<key>.contentDir` 配置 |
| 中文页面链接跳到英文页面 | 模板直接写了 `{{ .URL }}` 或硬编码 `/tags/` | 改用 `url` 辅助函数（见 §4.4） |
| `T "key"` 原样输出 | 当前语言和顶层 `strings` 都没有该 key | 补 `strings` 配置 |
| 翻译互链不出现 | 两边 `translationKey` 不一致，或一方是草稿 | 对齐 key；`--drafts` 构建 |
| `gobin check` 报 language key 冲突 | key 与 `page`/`tags`/`assets`/`public` 等保留段相同 | 换 key（如 `zh-cn`） |
| 删除了某语言后 `public/<lang>/` 还在 | 用了 `--clean=false` 增量构建 | 跑一次默认的 clean 构建 |
