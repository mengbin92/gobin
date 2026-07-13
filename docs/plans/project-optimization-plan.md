# Gobin 项目优化技术方案

## 1. 文档目标

本文档用于统一 Gobin 项目的优化方向、实施范围、阶段目标与验收标准，作为后续版本迭代、代码重构和发布管理的依据。

当前项目已经具备基础的静态站点生成能力，但在功能边界、实现一致性、生成正确性和工程可维护性上仍存在明显短板。优化工作的核心目标不是继续横向堆叠功能，而是先建立稳定、可信、可演进的生成器内核。

## 2. 当前状态评估

### 2.1 已具备能力

- 提供基础 CLI，支持 `build`、`serve`、`init`、`version`
- 支持 Markdown + YAML Front Matter 解析
- 支持基础列表页、文章页、标签页、分类页生成
- 支持 Sitemap、RSS、Atom、搜索索引生成
- 支持主题模板和静态资源复制
- 已建立基础单元测试

### 2.2 主要问题

#### 1. 产品承诺与实际实现不一致

README 中存在以下问题：

- 声称支持并行处理、增量构建、多语言、短代码、完整 Jekyll 兼容
- 文档列出了 `new`、`check` 等未实现命令
- 示例命令包含未实现参数

影响：

- 降低项目可信度
- 增加使用者试错成本
- 给后续版本规划带来认知混乱

#### 2. URL 与 permalink 体系未完成

- 配置层已定义 `permalinks`
- 实际生成时仍使用固定 `/<slug>/` 规则
- 无法支撑 README 中宣称的 Jekyll 风格链接兼容

影响：

- SEO 与历史链接兼容性不足
- 无法支撑多层级文章路径
- 主题中的相对链接在深层路径下容易失效

#### 3. 内容可见性规则缺失

- 解析层包含 `draft`、`published` 字段
- 生成阶段没有统一过滤逻辑
- 搜索、feed、页面输出对草稿处理不一致

影响：

- 草稿内容可能误发布
- 各类输出结果不一致
- 后续扩展发布日期、私有内容时难以演进

#### 4. 模板与输出体系不统一

- 页面有的走模板，有的直接拼接 HTML
- taxonomy 页面与默认模板使用不同资源路径
- 部分 partial 已存在但未纳入模板加载

影响：

- 主题覆盖不完整
- 页面风格和能力割裂
- 功能接入成本高，测试复杂

#### 5. 生成器职责过于集中

- `internal/generator` 同时承担排序、过滤、模板、taxonomy、资源复制、feeds、sitemap、压缩等职责

影响：

- 可维护性下降
- 变更影响面过大
- 很难建立清晰的测试边界

#### 6. 开发服务器能力描述与实现不一致

- `serve` 描述中宣称支持 LiveReload 注入
- 实际逻辑仅实现文件监听与重建

影响：

- 使用预期与实际行为不一致
- 后续扩展中间件能力时结构不够清晰

## 3. 优化目标

### 3.1 总体目标

将 Gobin 从“可运行的原型生成器”优化为“行为正确、接口清晰、可持续扩展”的静态站点生成器。

### 3.2 具体目标

- 保证文档、CLI、配置、输出结果一致
- 建立统一的文章可见性与 URL 生成规则
- 统一模板、taxonomy、静态资源输出路径
- 降低生成模块耦合度，为下一阶段重构做准备
- 补充关键回归测试，防止行为再次漂移

## 4. 分阶段实施方案

### 第一阶段：行为收口与正确性修复

#### 目标

先修复最直接影响项目可信度和发布正确性的部分。

#### 范围

- 收口 README，明确“已支持能力”和“规划能力”
- 删除文档中未实现命令和参数描述
- 实现基于配置的 permalink 生成
- 建立统一的文章可见性规则
- 默认过滤 `draft` 和显式 `published: false` 的内容
- 为 `build` / `serve` 增加草稿开关
- 统一默认模板、taxonomy 页面、初始化模板的静态资源路径
- 修复模板加载缺口，确保已有 partial 能被正常解析

#### 输出物

- 第一阶段代码修复
- 第一阶段回归测试
- README 更新

#### 验收标准

- README 中不存在未实现命令说明
- `permalinks.posts` 对文章 URL 生效
- 默认构建不输出草稿与未发布文章
- `--drafts` 可显式包含草稿
- taxonomy 页面样式路径与主模板保持一致
- 默认模板构建不再因缺失 partial/template func 失败

### 第二阶段：生成器结构重构

#### 目标

拆解 `generator` 的职责边界，减少后续功能接入成本。

#### 范围

- 拆分 `render`、`taxonomy`、`assets`、`feeds`、`permalink` 等子模块
- 取消直接拼接 HTML 的页面生成方式
- 所有页面统一通过模板渲染
- 引入统一页面数据结构

#### 验收标准

- taxonomy 页面通过模板渲染
- `generator.go` 不再承载大部分具体生成细节
- 新增页面类型可在不修改主流程的情况下接入

### 第三阶段：开发体验与主题系统增强

#### 目标

提升本地开发效率，并让主题系统成为真正可扩展接口。

#### 范围

- 明确 `serve` 的能力边界
- 正式实现 LiveReload 或调整为“自动重建”能力
- 改造为可组合的 HTTP mux / middleware 结构
- 统一主题覆盖规则与 fallback 机制

#### 验收标准

- `serve` 描述与行为完全一致
- 文件变更后的反馈链路清晰可见
- 主题覆盖顺序明确并可测试

### 第四阶段：能力补齐与性能优化

#### 目标

在正确性和结构稳定后，再补齐高价值能力。

#### 范围

- Markdown 扩展能力增强
- SEO/评论/统计配置真正接入
- 增量构建
- 并行构建
- 图片与资源优化

#### 验收标准

- 新增能力有明确配置入口和回归测试
- README 与实现同步更新
- 构建性能优化有可量化指标

## 5. 第一阶段详细设计

### 5.1 文档收口策略

README 按以下结构重写：

- 已支持
- 当前限制
- 规划中

原则：

- 不再描述未落地能力为“已支持”
- 未实现功能统一迁移到路线图或优化方案
- CLI 示例仅保留当前可执行命令和参数

### 5.2 permalink 设计

文章 URL 生成规则统一由生成阶段处理，而非解析阶段硬编码。

支持的占位符：

- `:year`
- `:month`
- `:day`
- `:title`
- `:slug`

默认规则：

- 当 `permalinks.posts` 未配置时，仍保持 `/<slug>/`

设计原则：

- 输出必须以 `/` 开头并以 `/` 结尾
- 多余 `/` 需要归一化
- 标题与 slug 必须进行 URL 安全化处理

### 5.3 内容可见性规则

统一规则如下：

- `draft: true` 默认不参与输出
- `published: false` 默认不参与输出
- 当执行 `build --drafts` 或 `serve --drafts` 时，允许输出草稿
- `published: false` 仍应保持不输出

说明：

`published` 必须区分“未配置”和“显式 false”，避免零值语义导致全部内容被过滤。

### 5.4 静态资源路径策略

默认资源 web 路径统一采用：

- `/assets/...`

为兼容已有示例站点和模板，可在生成期探测常见样式入口：

- `assets/css/main.css`
- `assets/css/style.css`

模板和 taxonomy 页面统一通过辅助函数获取样式路径，不再在模板内硬编码多个不同版本。

### 5.5 模板加载策略

模板加载范围应至少覆盖：

- `_default/single.html`
- `_default/list.html`
- `_default/404.html`
- `partials/header.html`
- `partials/footer.html`
- `partials/comments.html`
- `partials/analytics.html`

同时补充模板函数：

- `default`
- `absURL`
- `stylesheetPath`

## 6. 测试方案

### 6.1 单元测试

- permalink 生成测试
- 草稿/未发布过滤测试
- 样式路径选择测试
- 模板加载测试

### 6.2 集成测试

- 初始化站点后执行构建
- 验证首页、文章页、taxonomy 页能正确生成
- 验证生成结果中的资源引用真实存在

## 7. 风险与应对

### 风险 1：调整 URL 规则导致现有示例或模板链接失效

应对：

- 保持默认规则兼容旧行为
- 将站内导航改为绝对路径
- 为 permalink 行为增加测试

### 风险 2：模板函数扩展后影响旧模板解析

应对：

- 新增函数采用增量方式，不替换现有函数语义
- 保持旧模板可继续运行

### 风险 3：内容过滤规则改变影响已有站点输出数量

应对：

- 在 README 中明确默认行为
- 通过 `--drafts` 提供显式开关

## 8. 里程碑与交付建议

### M1

完成第一阶段，解决文档失真、URL 规则、可见性规则和资源路径统一问题。

### M2

完成 generator 结构拆分，taxonomy 页面全部模板化。

### M3

完成 `serve` 能力收口与主题系统增强。

### M4

完成能力补齐与性能优化。

## 9. 当前实施进展（更新于 2026-03-28）

### 9.1 已完成事项

#### 第一阶段已完成

- README 已按当前真实能力收口，移除了未实现命令和误导性描述
- `build` / `serve` 已支持 `--drafts`
- 已建立统一文章可见性规则，默认过滤 `draft: true` 和显式 `published: false`
- `permalinks.posts` 已参与文章 URL 生成
- 默认模板、taxonomy 页面、初始化模板的样式路径已统一
- 模板辅助函数与 partial 加载缺口已补齐

#### 第二阶段已完成的主体工作

- `internal/generator` 已按职责拆分为 `posts`、`taxonomy`、`templates`、`assets`、`site_files`、`output_controls`、`cleanup` 等模块
- taxonomy 页面已全部改为模板渲染，不再手写字符串 HTML
- 默认模板已统一到 `base` 布局
- 页面数据已开始收敛到 `BasePageData`、`ListPageData`、`SinglePageData`、`NotFoundPageData`
- `official-website` 主题已补齐自己的 `taxonomy` 与 `404` 模板，不再依赖默认回退
- `404.html` 已正式纳入生成流程
- 页面生成入口已统一为“页面注册表 + 统一渲染”模式
- `generator.go` 已进一步收敛为总编排器，首页、文章页、404、taxonomy 都通过统一页面规格生成

#### 站点级产物策略已完成

- `robots.txt` 已纳入生成流程
- feed / sitemap / search / robots 已支持显式配置开关
- feed / sitemap 仅在 `baseURL` 为绝对地址时生成
- `robots.txt` 中的 `Sitemap` 仅在绝对 `baseURL` 时输出
- 构建前输出目录清理策略已落地，`build` / `serve` 默认启用 `--clean`
- feed / sitemap / search / robots 已收敛为“产物注册表 + 统一执行”模式
- alias / assets / minify 也已纳入同一套产物执行链路

#### URL 兼容能力已新增

- 文章 front matter 中的 `aliases` 已正式接入生成流程
- alias 支持目录式路径和 `.html` 路径
- alias 页面会输出 `canonical`、`meta refresh`、`location.replace()` 和 `noindex`
- 已增加最小冲突保护，避免 alias 覆盖已生成真实页面

### 9.2 已完成验证

- `go test ./...` 已通过
- 已完成多轮 `init -> build` 冒烟测试
- 已验证默认模板和 `official-website` 主题都能正确输出首页、文章页、taxonomy 页和 `404.html`
- 已验证在不同 `outputs` 配置下，feed / sitemap / search / robots 能按预期生成或跳过
- 已验证 alias 页面能正确落盘并跳转到文章 permalink
- 已补充默认站点整站 golden 测试，锁定首页、分页页、文章页、taxonomy 页、`404.html`、alias 页面、`search-index*.json`、`robots.txt`、`index.xml`、`index.atom`、`sitemap.xml` 和复制后的 `css/main.css`
- golden 测试已对 feed / sitemap 中的动态时间字段做归一化处理，其余内容按实际产物逐文件比对
- 已补充 `official-website` 主题集成测试，覆盖首页、文章页、taxonomy 页、`404.html` 和主题静态资源复制
- 已修复 `official-website` 主题文章页中 taxonomy 链接未 `urlize`、标签链接缺少尾部 `/` 的问题
- 已补齐 `official-website` 主题自身的 favicon 资源，消除主题独立使用时的静态资源断链
- 已补充 `outputs` 配置组合测试，验证 feed / sitemap / robots 禁用时不会生成，search 启用时仍可独立输出
- 已补充 `official-website` 主题下的 alias 集成测试，验证主题场景中的 alias 页面生成与跳转行为
- 已补充 `official-website` 主题下的 drafts 集成测试，验证默认过滤、`--drafts` 放开草稿和 `published: false` 持续过滤的组合行为
- 已修复生成器在站点根 `assets/` 缺失时仍强制复制静态资源的问题，主题独立站点现在可以正常构建
- 已收口 `serve` 的能力边界，明确当前能力为“静态文件服务 + 文件监听自动重建”，不包含真正的 LiveReload 注入
- 已移除 `serve` 中未实现的 LiveReload 路由和占位代码，避免实现与描述继续漂移
- 已补充 `serve` 命令测试，覆盖 HTTP 文件服务、watch 路径收集、事件触发规则和 watcher 目录过滤逻辑
- 已让 `serve` 的 watcher 覆盖当前主题的 `layouts` 和 `assets` 目录，主题开发时会参与自动重建
- 已收口主题模板覆盖顺序，明确当前规则为“站点 `templates/` 优先，主题 `layouts/` 仅作 fallback”
- 已让样式入口探测与静态资源覆盖规则保持一致，站点 `assets/` 优先于主题 `assets/`
- 已补充主题覆盖顺序和 fallback 测试，锁定模板加载顺序与样式路径选择行为
- 已补充 partial 覆盖测试，验证站点 partial 可覆盖主题 partial，缺失时可回退到主题 partial
- 已补充静态资源同名覆盖测试，验证站点资源会覆盖主题中的同路径资源
- 已补充 `official-website` 主题下的 custom permalink 集成测试，验证深层文章路径与 taxonomy 跳转行为
- 已补充 `official-website` 主题下的 relative `baseURL` 集成测试，验证 feed / sitemap 跳过、search 保持生成、`robots.txt` 不输出 sitemap
- 已补齐 `official-website` 主题对 `SEO.Image`、`comments` 和 `analytics` 的接入，避免主题场景下这些能力静默失效
- 已补充 `official-website` 主题下的 SEO / comments / analytics 集成测试，验证 OG 图片、Twitter 图片、Disqus 评论和 Google Analytics 输出
- 已补充 `official-website` 主题下的 `comments/analytics + custom permalink` 组合测试，验证深层文章路径下第三方接入引用仍然正确
- 已补充 `official-website` 主题下的 `comments/analytics + drafts` 组合测试，验证草稿默认过滤、启用草稿后再接入第三方能力
- 已修复 `official-website` 主题文章模板在缺失 `description` 时误引用 `.Date` 导致主内容空渲染的问题
- 已补充 `official-website` 主题下的 `SEO/analytics + pagination` 组合测试，验证分页页仍保留 canonical、SEO 图片与 analytics 输出
- 已补充 `official-website` 主题下的 `comments/analytics + outputs` 组合测试，验证关闭站点级产物不会误伤页面级第三方接入
- 已补充 `official-website` 主题下的 `SEO/analytics + taxonomy` 组合测试，验证 taxonomy 索引页和详情页仍保留 SEO 图片与 analytics 输出
- 已补充 `comments` 作用域测试，验证评论只出现在文章页，不会误渲染到首页、列表页和 taxonomy 页
- 已补充主题分页导航结构测试，并修复 `official-website` 主题列表页未渲染分页导航的问题
- 已补充 `official-website` 主题空站点测试，验证无文章时首页仍可正常输出且不会生成多余分页页
- 已补充空 taxonomy 索引测试，验证无标签/分类时依然生成空态页面并输出预期文案
- 已补充空站点 `404.html` 测试，验证无内容场景下仍能生成主题版 404 页面
- 已补充主题缺失 `single.html` 时的 fallback 测试，验证站点模板可补齐主题缺失局部模板
- 已补充最小模板集测试，验证仅提供基础模板和 partial 时仍可完成首页、文章页、taxonomy 页和 `404.html` 输出
- 已为 `official-website` 主题补充核心整站 golden 基线，锁定首页、分页页、文章页、taxonomy 页、`404.html` 和主题静态资源输出
- 已将 `official-website` 主题核心场景从零散断言收敛为逐文件 golden 对比，降低后续主题回归漂移风险
- 已补充 `alias + taxonomy + custom permalink` 组合测试，验证主题 taxonomy 页面始终链接 canonical URL，不会混入 alias 路径
- 已收口首页与分页页的页面元信息标题规则，避免首页输出 `Site - Site`，并让分页页输出独立的 `<title>` / `og:title` / `twitter:title`
- 已补充首页与分页页元信息标题回归测试，并同步更新默认站点与 `official-website` 主题的 golden 基线
- 已补充 `official-website` 主题下的 alias 冲突集成测试，验证 alias 不会覆盖已生成的 taxonomy 页面，并在冲突时明确返回错误
- 已补充主题缺失 `taxonomy.html` 和 `404.html` 时的 fallback 测试，验证站点模板可补齐主题缺失页面类型
- 已补充生成级 fallback 测试，验证主题缺失 `taxonomy` / `404` 页面时，整站输出会真实回退到站点模板而不是只停留在模板加载层
- 已补充 `config.yaml` / `config.yml` / `_config.yml` / `_config.yaml` 的默认配置发现逻辑，`build` / `serve` 可直接识别 Jekyll 风格 `_config.yml`
- 已补充 front matter 字符串列表兼容，`tags` / `categories` / `keywords` / `aliases` 现在既支持数组，也支持单个字符串与逗号分隔字符串
- 已用真实博客 `../mengbin92.github.io` 做构建验证，当前已能识别 `_config.yml` 并解析 607 篇文章，但会在模板阶段停在 `no templates found`，说明当前边界仍是 Gobin 只支持自己的 `templates/` 体系，不直接兼容 Jekyll 的 `_layouts` / `_includes`

### 9.3 当前阶段结论

到目前为止，第一阶段目标已经完成，第二阶段的核心目标也已经基本落地。当前项目已经从“功能可跑的原型”进入“生成行为基本一致、主流程已收敛、具备持续演进与回归保护基础”的状态。

当前剩余的高价值工作，已经从“主流程收敛”转向“少量高价值新增场景补齐”和“已有回归基线的持续维护”。

### 9.4 下次继续建议

建议下次从以下两项中选择其一继续：

- 继续维护 golden / 集成测试基线，只补少量仍然容易漂移的主题场景
- 精选少量仍未锁住的高价值内容组合继续补齐，例如更极端的空态或更细的 partial / assets fallback 边界
- 如果要继续推进真实博客兼容，下一步应单独立项评估 `_layouts` / `_includes` 到 Gobin `templates/` 的迁移或兼容策略，而不是再继续零散补点

推荐优先级：

- 先维护并压实已有 golden / 组合测试基线
- 再按需要补少量剩余高价值组合测试与文档同步

原因：

- 当前构建主流程、主题主路径、`serve` 基础能力以及核心主题基线都已经有了回归保护
- 接下来更值得做的是只补那些仍然容易漂移、但尚未被锁住的高价值场景
- 这样可以控制测试面继续膨胀带来的维护成本

## 10. 结论

Gobin 当前最需要的不是继续扩展功能面，而是先建立“行为正确、一致、可验证”的基础层。第一阶段完成后，项目才能从原型状态进入可持续演进状态；第二阶段以后再做结构和性能优化，收益才会真正稳定沉淀下来。
