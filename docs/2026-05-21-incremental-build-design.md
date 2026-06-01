# Gobin 增量构建设计文档

- 文档版本：2026-05-21
- 阶段：A 阶段（增量构建主线）
- 状态：草稿，待 review 后开始实现

## 1. 背景

当前 Gobin 已经具备如下"伪增量"能力（v1.1.0）：

- 静态资源复制：未变化的源文件不会重写目标（基于 size / mode / mtime / 内容比对）
- 页面渲染：每个 `PageSpec` 渲染完成后会与已存在的输出文件做 `bytes.Equal`，相同则跳过 `os.WriteFile`，统计为 `rendered/skipped`

这两条让"重复构建"的磁盘写入接近零，但仍然付出全量代价：

- 每篇 post 都会被 `ParsePost` 解析（YAML + Markdown + 摘要 + 字数）
- 每个 `PageSpec` 都会执行模板渲染并生成完整 HTML 字节
- aggregate 类（feeds、sitemap、search index、alias）每次都遍历完整 `[]*parser.Post`

A 阶段的目标：**把"内容/配置未变化"的单文章页和聚合产物的工作完全跳过**，把重复构建从"全量解析 + 全量渲染 + 0 次写"压缩到"按需解析 + 按需渲染 + 0 次写"。

## 2. 现有依赖关系

以一次"修改一篇 post 源文件"事件为例，下游受影响的 artifact 如下：

| Artifact | 触发条件 |
| -- | -- |
| 单篇 post `index.html` | 源文件内容或元数据变化 |
| `index.html` 与翻页 | 列表里出现/消失了某条目，或某条目的 Title/Description/Date 变化 |
| `tags/<tag>/index.html` | 某 post 的 tag 列表发生增删，或 post 在该 tag 下的元数据变化 |
| `categories/<cat>/index.html` | 同上，但针对 category |
| RSS / Atom (`feed.xml`, `atom.xml`) | 任意已发布 post 的 Title / Description / Content / Date 变化 |
| `sitemap.xml` | 任意 post URL 增删，或 lastmod 变化 |
| `search-index.json` / `.min.json` | 任意 post 的 Title / Description / Tags / Categories / Content 变化 |
| 404 / standalone pages | 单独走 standalone page 路径，受其源文件变化驱动 |
| Alias 重定向 | 当 post 的 aliases 字段发生增删 |

观察：所有 aggregate 产物的 invalidation 都可以归结为"任一 post 的相关字段发生变化"。如果我们能给每个 post 维护一份 `参与 aggregate 计算的字段哈希`，那么 aggregate 只需要看"这次 build 的 post 集合的聚合哈希"和上一次的差异，就能决定是否要重跑。

## 3. 不在本阶段处理的事项

- 模板和配置变化的影响：本阶段仅处理"内容输入"变化，模板/配置/主题变化时一律 invalidate 所有缓存（直接退化为全量）
- 并行构建：留给 P3 阶段独立处理
- 静态资源管线已经有 manifest + stale 清理，本阶段不动它
- HTML/CSS minify 流程不参与增量计算，启用时仍走全量

## 4. Manifest 设计

新增 `<publishDir>/.gobin-build.json`，结构如下：

```json
{
  "version": 1,
  "build_env_hash": "<hash of: config + theme dir + templates dir + parser render options>",
  "posts": [
    {
      "source_path": "_posts/2026-01-01-hello.md",
      "source_hash": "<sha256 of file bytes>",
      "output_path": "2026/01/01/hello/index.html",
      "aggregate_hash": "<sha256 of: title|description|date|tags|categories|content>"
    }
  ],
  "pages": [
    {
      "source_path": "pages/about.md",
      "source_hash": "<sha256 of file bytes>",
      "output_path": "about/index.html"
    }
  ]
}
```

要点：

- `build_env_hash` 是任何"全局影响"的指纹：模板目录任一文件 mtime 改变、config 改变、主题改变、parser render options 改变，都让它变化。一旦不匹配上次记录，直接降级为全量构建。
- `source_hash` 决定"这篇 post 是否需要重新 ParsePost + 渲染单页"。
- `aggregate_hash` 决定"这篇 post 是否对 aggregate 产物有贡献变化"。两个 hash 分开的原因：post Content 改变会让两个 hash 都变，但 `published: false` 切换、grammar tweak 等可能只影响 single page、不影响 aggregate；反之亦然。第一版我们把 `aggregate_hash` 直接等于 `source_hash` 来简化实现，并在文档里说明日后可以收紧。
- post 顺序按 `source_path` 排序写入，保证 manifest 在文件层 diff-friendly。

## 5. 跳过规则

### 5.1 单文章页

执行 `--clean=false` 时：

1. 读取上次 manifest（不存在或 `build_env_hash` 变化时视为不存在）
2. 对当前 `_posts/` 目录下每个文件：
   - 计算 `source_hash`
   - 在 manifest 中查找上次记录
   - 若 `source_hash` 相同 **且** 上次记录的 `output_path` 仍存在于磁盘 **且** 不在 `--drafts` 切换边界上 → 标记为 `skipPostRender`
3. `skipPostRender` 的 post 不进入 `ParsePost`；它的 `PageSpec` 直接跳过模板渲染（统计为 skipped）
4. 其它 post 走原全量路径

对 standalone page 同理。

### 5.2 列表与翻页

按 `posts` 排序（已统一是 date desc），把"参与列表的最小元数据"打包成一个 hash：

```
posts_list_hash = sha256(json([{title, date, url, summary, draft, published} for post in posts]))
```

- 如果 `posts_list_hash` 与上次相同 **且** `index.html` / 翻页文件齐全，则不渲染列表页
- 否则全量渲染列表（这里不细分到"哪页变了"，因为翻页边界很容易被一次插入打乱）

### 5.3 Tag / Category 页

按 tag/category 各自维护一个 `taxonomy_hash`：

```
tag_hash[t] = sha256(json([{title, date, url, summary} for post in posts_in_tag(t)]))
```

- 仅当 `tag_hash[t]` 变化时重建 `tags/<t>/index.html`
- 同时维护 `tags_index_hash`（tag 列表本身的指纹），变化时重建 `tags/index.html`

### 5.4 RSS / Atom / Sitemap / Search

四个 aggregate 都按"参与 post 集合的聚合 hash"判断：

```
feed_hash    = sha256(join(post.aggregate_hash for post in published_posts))
sitemap_hash = sha256(join(post.url|lastmod for post in posts))
search_hash  = sha256(join(post.aggregate_hash for post in published_posts))
```

第一版：把 feed_hash / search_hash 都等于 `aggregate_hash` 累加，sitemap 单独算一份。一旦上次值与本次相同 → skip 写盘。

### 5.5 Aliases

aliases 跟单文章页绑定：post 的 `aliases` 字段在 `source_hash` 中已经覆盖，因此 single page skip 时也跳过 alias 重写。alias 缓存独立存一份"alias → target"映射，当 single page 进入 skip 路径时复用上次的 alias 列表。

## 6. CLI 与默认行为

- `gobin build` 默认行为不变（`--clean=true`），相当于一次"冷构建"，会清空输出目录并丢弃 manifest 缓存
- 新增 `--incremental` 标志（互斥于 `--clean`），等价于 `--clean=false` + 启用 manifest 跳过逻辑
- `gobin serve` 的 watcher 默认每次重建会走增量路径（rebuild 不应清目录），相当于自动开启 `--incremental`
- 如果 manifest 不存在或 `build_env_hash` 变化，所有 skip 路径自动降级为全量，并在 stdout 提示原因

## 7. 渐进交付策略

按以下顺序分 commit，每一步独立可验证：

1. `A.2.1` Build manifest schema 与读写（暂不接入 skip 路径，先保证 read/write round-trip）
2. `A.2.2` 计算并写入 source_hash / aggregate_hash / build_env_hash，但渲染流程仍是全量（manifest 作为副产物落地）
3. `A.3.1` 把"上次 manifest 中 source_hash 未变 + 输出存在"的单文章页跳过 ParsePost
4. `A.3.2` standalone page 同样处理
5. `A.4.1` 列表 / 翻页跳过
6. `A.4.2` taxonomy 跳过
7. `A.4.3` feed / sitemap / search 跳过
8. `A.5` benchmark + 文档同步

每一步完成后跑 `go test ./... -race`，并补一两个"修改一篇 post → 期望哪些产物被重渲染、哪些被跳过"的集成测试。

## 8. 风险与缓解

- **风险**：模板/资源变化时漏 invalidate，导致输出"看起来对、实际过期"。
  缓解：`build_env_hash` 覆盖模板目录全部文件 + 主题目录文件 + config 字段；任意一个变化都全量。
- **风险**：manifest 损坏。
  缓解：JSON 解析失败时直接当作不存在处理，自动降级全量。
- **风险**：与 `--clean=true` 配合的语义混乱。
  缓解：`--clean=true` 既清 manifest 又清输出目录；`--incremental` 隐含 `--clean=false`。
- **风险**：测试可能会偶发不稳定（依赖 mtime 等）。
  缓解：所有跳过决策只用内容哈希，不读 mtime。

## 9. 验收

- `gobin build --incremental` 在二次构建（无变化）时：
  - 单文章页全部 skipped
  - aggregate 全部 skipped
  - 静态资源全部 skipped
- 修改一篇 post 后第二次 `gobin build --incremental`：
  - 该 post 单页 rendered = 1
  - 列表 / 受影响的 tag / category / feed / search / sitemap rendered
  - 其它 post 单页 skipped
- `gobin serve` watch 模式触发 rebuild 时复用同一套增量路径
- benchmark 显示"二次构建（无变化）"耗时显著低于"首次构建"

## 10. 后续可演进项

- `aggregate_hash` 收紧：分离"对 feed 有效字段"和"对 search 有效字段"（已完成，见 v2 manifest 的 per-category 哈希）
- 配合并行构建（P3）（已完成，见 `gobin build --jobs`）
- `serve` 中根据变化文件集做"亚秒级 partial rebuild"（已完成，见下）

## 11. serve 亚秒级 partial rebuild（2026-06-01）

此前 `gobin serve` 的每次 watch 重建都会调用 `loadSiteBuildInput()`，
对站点内**全部** markdown 重新读取、解析并渲染 HTML（`O(N)` 解析），即便
增量构建已跳过未变化页面的重新渲染。watcher 虽然知道哪个文件变化
（`event.Name`），却把这个信息丢弃了。

现在 watcher 子系统内维护一个会话级解析缓存（`contentCache`）和变更集
（`changeSet`）。每个通过 `shouldRebuildForEvent` 的事件经 `classifyChange`
归类后记入变更集；重建时 `newIncrementalLoader` 只重新解析变化的内容文件，
其余从缓存复用：

- **首次重建 / 结构性变更 / 无内容变更** → 全量 `loadSiteBuildInput`
  并刷新整个缓存。结构性变更指 config 文件、`templates/`、主题目录，或内容
  目录下的非 markdown 文件。
- **内容 / 页面 markdown 变更** → 仅重新解析这些文件。变更路径若已不存在
  （删除、编辑器原子保存的 rename-over）则从缓存移除，以磁盘最终状态为准。
- **静态资源变更** → 不触碰解析缓存，交由生成器的增量资源复制处理。

正确性保证：

- 缓存在 `assemble()` 时按 `FilePath` 字典序输出**结构体浅拷贝**。浅拷贝避免
  生成器 `preparePosts` 的原地改写（`URL`/`Content`/`ContentHTML`/`Summary`）
  在多次重建间累积；字典序复现 `filepath.WalkDir` 的解析顺序，使等日期文章与
  独立页面的产物保持字节一致。
- 解析仅在全部成功后提交到缓存，半写文件的瞬时解析错误不会污染上次的良好状态。

已知 v1 限制：watch 模式以 `cleanOutput=false` 运行，删除一篇文章会留下其
旧的单页 HTML（与本次优化前行为一致，非回归）；按变更集精确清理 stale 输出
留作后续项。
