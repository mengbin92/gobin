# 增量构建使用指南

> Gobin v1.4.0 起随 `--incremental` 发布。本指南面向站点作者与 CI 维护者，解释什么时候用、产物怎么跳、什么时候会回退全量。

## 1. 解决什么问题

默认 `gobin build` 每次都会：

- 解析所有 `_posts/*.md` 的 YAML + Markdown + 摘要 + 字数；
- 渲染每篇文章页、列表页、taxonomy 页、404；
- 重写 feed / atom / sitemap / search index / aliases / robots 等聚合产物。

对 100+ 篇文章的站点，每次都跑一遍很慢，而**日常编辑只改 1 篇**。`--incremental` 配合 `--clean=false` 让 Gobin 把"未变化"的工作完全跳掉，只渲染/重写真正受影响的产物。

## 2. 快速上手

```bash
# 第一次：完整构建出基线
gobin build

# 之后：跳过未变化的产物
gobin build --incremental --clean=false
```

行为约定：

- `--incremental` **必须**与 `--clean=false` 共存；与 `--clean=true` 组合会被构建器拒绝：

  ```
  --incremental cannot be combined with --clean=true; pass --clean=false
  ```
- `gobin serve`（watch 模式）**自动启用** `--incremental`，无需手写标志。
- 静态资源复制本身就有 `size / mtime / 内容` 三重判定，重复构建不会无谓写盘，与 `--incremental` 正交叠加。

## 3. 跳过哪些产物

Gobin 在 `publishDir/.gobin-build.json`（manifest 文件）里记录两类指纹：

- **单文件源指纹**：每个 post / standalone page 的源文件内容 hash；
- **聚合指纹**：所有 post / page 的关键字段（Title / Description / Date / Tags / Categories / URL / Content）的集合 hash，以及由它们派生的 feed / sitemap / search index / aliases 聚合 hash。

重建时按下面表格决定跳 / 不跳：

| 产物 | 触发重写的条件 |
|------|----------------|
| 单篇文章 `index.html` | 该 post 源内容指纹变化（YAML + Markdown 字节级） |
| 列表页（含分页） | 任何 post 的 Title / Description / Date 变化，或文章集合增删 |
| `tags/<tag>/index.html` | 该 tag 下的 post 集合或其中任一 post 的关键字段变化 |
| `categories/<cat>/index.html` | 同上，针对 category |
| `feed.xml` / `atom.xml` | 任何已发布 post 的 Title / Description / Content / Date 变化 |
| `sitemap.xml` | 任何 post URL 增删，或 lastmod 变化 |
| `search-index.json` / `.min.json` | 任何 post 的 Title / Description / Tags / Categories / Content 变化 |
| 404 / standalone pages | 由对应源文件指纹驱动 |
| Alias 重定向 | aliases 字段增删 |

未触发的产物**根本不会被渲染**（而不是渲染了再 diff），统计上记为 `Skipped`。

## 4. 什么时候会回退全量

manifest 里还有第三个维度——**env hash**。它覆盖：

- `config.yaml` 全部字段
- 主题目录、`templates/`、`_layouts/` 与 `_includes/` 目录的文件清单 + 内容指纹
- 渲染选项（`RenderOptions`、短代码 registry hash）

只要 env hash 变化，Gobin 直接走全量构建，**不会**尝试用旧 manifest 做"分批跳过"。这避免了"模板改了但 manifest 没感知导致产物与模板脱节"的灾难。

具体会让 env hash 失配的常见改动：

- 改 `config.yaml` 任何字段（包括 SEO、permalink、pagination）。
- 改 `templates/`、`_layouts/`、`_includes/` 或主题 `layouts/` 下的任何模板文件。
- 新增 / 删除 / 重命名主题或短代码模板。
- 改 `markup.allowUnsafeHTML` 或 `markup.highlight` 配置。
- 升级 Gobin 本身（manifest version 不一致也会被忽略）。

## 5. 怎么观察是否真在跳

两种方式：

- 看 `BenchmarkBuildIncremental_NoChanges` 的方向：用 `make benchmark` 跑基准，对比 `BenchmarkBuildFull/posts=100` 与 `BenchmarkBuildIncremental_NoChanges/posts=100` 的 ns/op，前者通常 18 ms 量级、后者约 2 ms 量级。
- 临时给 build 加一层打印（已内置）：`gobin build --incremental --clean=false --verbose` 会在 stderr 看到每篇 post 命中/未命中的日志（`level=DEBUG`），并显示 `pages_skipped`、`artifacts_skipped` 的最终统计。

CI 抓取方式：

```yaml
- name: Build site
  run: gobin build --incremental --clean=false
- name: Inspect build log
  run: cat publishDir/.gobin-build.json | jq '.posts | length'
```

`.gobin-build.json` 是 JSON，可以直接 `jq`。

## 6. 与 `--jobs` / `--minify` 的关系

- `--jobs N`（页面渲染并发）和 `--incremental` **正交**：`--incremental` 决定要不要渲染一页，`--jobs` 决定渲染时启几个 worker。两边可以同时开。
- `--minify` 影响最终输出字节；与 `--incremental` 也正交，但启用 `--minify` 后产物内容变化时，manifest 也会被 invalidate（同 env hash 重新构建流程）。

## 7. 失效 manifest 的办法

- 删除 `<publishDir>/.gobin-build.json`：下一次 `--incremental` 会发现 manifest 不存在，自动降级为全量构建并写回新 manifest。
- 切换到 `--clean=true`（不带 `--incremental`）：清空 `publishDir` 后全量重建，manifest 也会被重写。
- 切换 Gobin 版本：manifest 的 schema version 字段失配，Gobin 自动忽略旧 manifest。

## 8. 排错

| 症状 | 可能原因 | 处理 |
|------|----------|------|
| 报错 `--incremental cannot be combined with --clean=true` | `--clean=true` 会清空 `publishDir`，manifest 也就没了 | 加 `--clean=false` 或干脆去掉 `--incremental` |
| 编辑一篇文章后所有页都重渲染 | env hash 变了（多半是改了配置或模板） | 正常行为；确认改动是不是只该影响 env 维度 |
| 删了一篇文章后旧 `index.html` 还在 `publishDir` | `--clean=false` 不删磁盘已有但本次没生成的产物 | 想清掉就 `gobin build --clean=true`；或者直接 `rm` 旧产物 |
| 想让搜索索引更新但 feed 没动 | 正常：两边各自判断 | 无需处理；下次任一相关字段变两边一起动 |
| `.gobin-build.json` 体积膨胀 | 10000+ 篇文章下 manifest 含每篇指纹 | 单篇指纹 ~100 B 量级；目前不在热路径上，不用担心 |
| CI 上 baseline 跑出来变慢了 | 可能换了一台 I/O 慢的 runner，env hash 经常失配 | 改 CI runner 类型 / 关掉某些随机时间戳字段 |

## 9. 基准参考

仓库自带 `benchmark-results.txt` 跟踪：

```
BenchmarkBuildFull/posts=100            ~18.3 ms
BenchmarkBuildIncremental_NoChanges/posts=100  ~2.1 ms
```

数字会随硬件浮动，但**比例**（约 6–10×）是稳定的。如果你的实测偏离这个数量级，检查上面 4 类 env hash 是否频繁失配。
