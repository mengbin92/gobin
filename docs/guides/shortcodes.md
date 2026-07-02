# Shortcodes 使用指南

> Gobin v1.4.0 起内置。本指南面向站点作者，说明在 Markdown 正文中如何调用、参数化、覆盖与排错短代码。

## 1. 短代码是什么

短代码（shortcode）让你在 Markdown 正文里写一个简短的指令，由 Gobin 渲染成一段结构化 HTML（figure、YouTube 嵌入、Gist、代码高亮块等）。它解决了两个常见痛点：

- 不必为插入一段固定 HTML 而打开全局 `markup.allowUnsafeHTML: true`。
- 同一段结构化片段（如图注、嵌入）在所有文章里只写一次、由模板统一控制。

语法向 Hugo 看齐：

| 形式 | 含义 |
|------|------|
| `{{< name args >}}` | HTML 形式：模板输出作为**原始 HTML** 注入；即使用户把 `markup.allowUnsafeHTML` 关掉也生效 |
| `{{% name args %}}` | Markdown 形式：模板输出**再走一次 Markdown 渲染**，适合需要 Markdown 语法的嵌套场景 |
| `{{< name >}}body{{< /name >}}` | 配对形式：正文通过 `.Inner` 提供；`body` 内部允许继续嵌套短代码 |

## 2. 参数语法

位置参数和引号命名参数可以混用，按出现顺序解析：

```markdown
{{< youtube dQw4w9WgXcQ >}}                              {{/* 位置参数 0 */}}

{{< figure src="/img/cover.png" alt="封面" caption="图 1" >}}  {{/* 命名参数 */}}

{{< figure "/img/cover.png" "封面" "图 1" >}}                  {{/* 三个位置参数 */}}
```

模板里通过 `.Get` 统一读取：

- `.Get 0` 读第 0 个位置参数（int key）。
- `.Get "src"` 读命名参数 `src`（string key）。
- 命名参数必须用双引号包裹，值里可以 `\"` 转义双引号。

## 3. 内置短代码

| 名称 | 签名 | 输出 |
|------|------|------|
| `figure` | 必填 `src`；可选 `alt` / `title` / `caption` / `link` | `<figure>` 块，可选外链包裹 |
| `youtube` | 视频 ID（位置参数 0 或命名 `id=`） | 懒加载 `<iframe>` |
| `gist` | GitHub 用户 + gist ID（位置 0/1 或命名 `user=` / `id=`） | `<script>` 嵌入 |
| `highlight` | 配对；位置参数 0 是语言名 | 带 `<pre class="language-…">` 的代码块 |

> 完整参数与源码见 `internal/shortcode/builtins.go`。所有内置短代码都可以被同名文件覆盖。

## 4. 自定义与覆盖

短代码模板是普通 Go `html/template` 文件。Gobin 按以下优先级合并，**先到先得**：

1. 站点 `templates/shortcodes/<name>.html`
2. 主题 `<theme>/layouts/shortcodes/<name>.html`
3. 内置（来自 `internal/shortcode/builtins.go`）

例：写一个 `note.html`，给文章加一个引用块：

```bash
mkdir -p templates/shortcodes
```

`templates/shortcodes/note.html`：

```html
<blockquote class="gobin-note">
  <p>{{ .Inner | safeHTML }}</p>
</blockquote>
```

在文章里：

```markdown
{{< note >}}这是要写进引用块里的内容，可以包含**Markdown**。{{< /note >}}
```

主题作者想覆盖，文件落在主题目录里：

```
themes/example/layouts/shortcodes/figure.html
```

Gobin 会用主题版本替换掉内置版本；如果作者同时在 `templates/shortcodes/figure.html` 放一份，站点级胜出。

## 5. 模板上下文

短代码模板可访问以下字段/方法：

- `.Name string` —— 短代码名
- `.Get key` —— int 走位置参数、string 走命名参数
- `.Inner string` —— 配对形式的正文，**已先行展开**嵌套短代码
- `.Page` —— 当前 `*parser.Post` 或 `*parser.Page`（在 `serve` 重建时也能用）

工具函数：

- `safeHTML` —— 标记字符串为安全 HTML（绕过 html/template 的转义）。
- `absURL` —— 用 `Site.BaseURL` 拼接绝对 URL。
- `urlize` —— 文本 → slug。
- `default "fallback" value` —— `value` 为空时回落到 `"fallback"`。

## 6. 失效与缓存

短代码模板位于 `templates/shortcodes/` 与主题 `layouts/shortcodes/`，天然被：

- 增量构建的 env hash 涵盖（`buildManifest` 会扫到，env 不匹配自动回退全量）；
- `serve` 的结构性变更分类涵盖（改短代码模板会被识别为"非内容变更"，回退全量重建）。

所以编辑短代码无需手动清理缓存、也无需新增配置项。

## 7. 排错

| 症状 | 可能原因 | 处理 |
|------|----------|------|
| 构建失败：`unknown shortcode "xxx"` | 引用了未注册且无内置的短代码 | 检查拼写；新建 `templates/shortcodes/xxx.html`；或确认主题 `layouts/shortcodes/xxx.html` 存在 |
| 短代码语法出现在输出里 | 它落在代码围栏或行内代码里 | 代码围栏（```` ``` ````）与 `` ` `` 行内代码内不会展开短代码，把示例搬出来 |
| `<figure>` 被转义成文本 | 用了 `{{% %}}` 而非 `{{< >}}` | `{{% %}}` 会把模板输出再当 Markdown 渲染，HTML 标记会被吃掉 |
| 模板里 `{{ .Inner }}` 输出 `&lt;` 等实体 | `Inner` 是 HTML 字符串，模板默认转义 | 改为 `{{ .Inner | safeHTML }}` |
| 修改短代码模板后产物没变 | 你跑的是上次 `gobin build` 的产物 | 加 `--incremental` 时也确保模板/主题路径没被忽略；`serve` 下会自动全量重建 |

## 8. 完整示例

`config.yaml`：

```yaml
markup:
  allowUnsafeHTML: false  # 默认值；短代码不依赖这个开关
```

某篇 `_posts/2026-06-02-demo.md`：

```markdown
---
title: "短代码演示"
date: 2026-06-02T10:00:00+08:00
tags: ["shortcode", "demo"]
---

# 短代码演示

封面图：

{{< figure src="/images/cover.png" alt="封面" caption="由短代码生成" >}}

嵌入一段 YouTube：

{{< youtube dQw4w9WgXcQ >}}

代码块（用内置 highlight 短代码）：

{{< highlight go >}}
package main

import "fmt"

func main() { fmt.Println("hello") }
{{< /highlight >}}

引用（自定义短代码）：

{{< note >}}这是一段引用文本。{{< /note >}}
```

构建：

```bash
gobin build
```

输出 HTML 里 `<figure>` / `<iframe>` / `<pre class="language-go">` / `<blockquote class="gobin-note">` 都会作为活动 HTML 出现，**而 `config.yaml` 里的 `allowUnsafeHTML` 仍然是 false**。
