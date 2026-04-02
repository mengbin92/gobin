# Jekyll `_layouts` / `_includes` 到 Gobin `templates/` 的兼容评估

## 1. 目标

评估 Gobin 是否应该直接兼容 Jekyll 的 `_layouts` / `_includes` 模板体系，或者应采用“迁移到 Gobin `templates/`”的方案，并给出推荐路线。

本评估基于以下真实项目样本：

- 站点：`../mengbin92.github.io`
- 配置：`_config.yml`
- 布局目录：`_layouts/`
- include 目录：`_includes/`

## 2. 现状结论

当前 Gobin 已经可以通过以下兼容改动推进到真实博客的模板阶段：

- 自动识别 `_config.yml`
- 解析 607 篇 `_posts`
- 兼容 front matter 中的字符串型 `tags` / `categories` / `keywords` / `aliases`

但构建最终会停在：

- `Error generating site: no templates found`

原因不是目录名不同这么简单，而是模板语言和数据模型都不同。

## 3. 真实博客的模板特征

从真实博客抽样可见：

### 3.1 布局继承依赖 Jekyll Front Matter

例如 `_layouts/default.html`：

- 自身带 front matter
- 通过 `layout: base` 继续继承上级布局
- 使用 `{{ content }}` 承接正文

这不是 Gobin 当前 `base + render MainTemplate` 体系的简单映射。

### 3.2 include 依赖 Liquid 语法和动态参数

例如 `_layouts/post.html`：

- `{% include header.html type="post" %}`
- `{% include toc.html html=content %}`
- `{% assign ... %}`
- `{% case %}`, `{% when %}`, `{% for %}`, `{% if %}`

这要求模板引擎支持：

- include 参数传递
- 局部变量赋值
- 分支和循环控制
- 过滤器管道

Gobin 当前基于 Go `html/template`，并不具备 Liquid 解释能力。

### 3.3 元信息依赖 Jekyll/Liquid 专有过滤器

例如 `_includes/head.html` 使用了：

- `strip_html`
- `xml_escape`
- `truncatewords`
- `absolute_url`
- `relative_url`
- `strip_index`
- `date_to_xmlschema`
- `default`
- `capture`

这已经不是单纯的模板文件查找问题，而是完整的 Liquid 运行时需求。

### 3.4 页面数据模型依赖 Jekyll 约定

真实模板大量引用：

- `site.*`
- `page.*`
- `layout.*`
- `page.previous`
- `page.next`
- `page.content`
- `page.id`

而 Gobin 当前的数据结构是：

- `BasePageData`
- `ListPageData`
- `SinglePageData`
- `TaxonomyPageData`

字段名、层次和可用上下文都不一致。

## 4. 两条可选路线

### 路线 A：直接兼容 Jekyll `_layouts` / `_includes`

含义：

- Gobin 继续保留当前生成核心
- 新增一层 Jekyll/Liquid 模板兼容运行时
- 让真实 Jekyll 站点无需迁移模板即可构建

优点：

- 对现有 Jekyll 站点最友好
- 真实博客迁移成本最低
- 对外“兼容 Jekyll”说法更完整

缺点：

- 实现成本高
- 需要引入 Liquid 解释能力，或自行实现一个不小的子集
- 需要维护 Gobin 数据模型到 Jekyll `site/page/layout` 视图模型的映射
- 后续 debug 和测试复杂度显著上升

工程风险：

- 高风险
- 容易把 Gobin 从“静态站点生成器”变成“Jekyll 兼容层”
- 一旦只兼容一部分 Liquid，用户预期会继续失真

结论：

- 不建议作为当前阶段主路线

### 路线 B：提供迁移方案，把 Jekyll 模板迁到 Gobin `templates/`

含义：

- Gobin 不解释 Liquid
- 提供模板迁移规范、兼容映射文档、必要脚手架
- 把 `_layouts` / `_includes` 映射为 Gobin 的：
  - `templates/_default/*.html`
  - `templates/partials/*.html`

优点：

- 保持 Gobin 架构清晰
- 风险可控
- 与当前 `templates`、theme fallback、golden 测试体系一致
- 更容易逐步验证和落地

缺点：

- 需要一次性迁移模板
- 老站点不能“零改动直接运行”

工程风险：

- 中风险
- 主要成本在模板翻译和部分语义补齐，不在核心架构

结论：

- 建议作为主路线

## 5. 推荐方案

推荐采用：

- 主路线：路线 B，模板迁移
- 辅路线：仅补少量“输入兼容”，不做 Liquid 运行时

具体边界建议：

- 继续兼容 `_config.yml`
- 继续兼容 Jekyll 常见 front matter 写法
- 不承诺直接兼容 `_layouts` / `_includes`
- 用文档明确说明：Jekyll 内容结构可复用，但模板需迁移到 Gobin `templates/`

这样能保证：

- 内容迁移成本低
- 模板边界清晰
- 不会把项目拖进长期的 Liquid 兼容泥潭

## 6. 最小可行迁移方案

建议单独拆成一个小项目，目标不是“自动兼容所有模板”，而是先让真实博客 `mengbin92.github.io` 可在 Gobin 下跑通。

### 阶段 1：建立迁移骨架

新增：

- `templates/_default/base.html`
- `templates/_default/list.html`
- `templates/_default/single.html`
- `templates/_default/404.html`
- `templates/_default/taxonomy.html`
- `templates/partials/header.html`
- `templates/partials/footer.html`
- `templates/partials/comments.html`
- `templates/partials/analytics.html`

做法：

- 先复刻当前博客最核心的页面结构
- 不追求所有 Jekyll 特性一次到位

### 阶段 2：映射核心能力

优先映射：

- 页面标题 / 副标题
- 文章日期 / tags / categories
- 评论 partial
- analytics partial
- 导航和 footer
- 列表页分页

这部分和 Gobin 现有能力高度重合，迁移成本可控。

### 阶段 3：处理 Jekyll 特有增强点

按必要性拆分：

- `page.previous` / `page.next`
- `before-content` / `after-content`
- `cover-img`
- `toc`
- 第三方统计脚本
- 自定义 CSS / JS 注入

原则：

- 只有真实博客确实在用、且价值高的能力才迁
- 其余能力不要为了“名义兼容”提前做复杂抽象

## 7. 不建议现在做的事

- 不建议引入完整 Liquid 解释器
- 不建议在 Gobin 内部同时维护两套模板模型
- 不建议把 `_layouts`、`_includes`、`templates` 三套查找规则混在一起
- 不建议宣称“完整兼容 Jekyll 模板”

## 8. 验收标准建议

如果单独立项，建议验收标准定成以下几条：

- `mengbin92.github.io` 可在 Gobin 下完成整站构建
- 首页、文章页、分页页、标签页、分类页、`404.html` 能正确生成
- 文章图片和静态资源路径不失效
- 评论和统计脚本至少能通过 partial 方式接回
- `go test ./...` 持续通过
- 为真实博客新增一条最小集成测试或验证脚本

## 9. 建议的下一步

如果确认单独立项，建议按下面顺序推进：

1. 先为 `mengbin92.github.io` 手工建立一套 Gobin `templates/` 骨架
2. 先跑通首页、文章页、分页页
3. 再补 taxonomy、`404`、comments、analytics
4. 最后再决定是否需要做半自动迁移脚本

## 10. 最终判断

结论很明确：

- “直接兼容 Jekyll `_layouts` / `_includes`”技术上可做，但不适合作为当前阶段主路线
- “迁移模板到 Gobin `templates/`”才是更稳、更可控、也更符合当前项目状态的方案

如果目标是尽快让真实博客落地到 Gobin，最佳路径不是实现 Liquid 兼容层，而是围绕真实博客建立一套 Gobin 原生模板，并把迁移过程文档化、测试化。
