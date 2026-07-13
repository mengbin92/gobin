# serve watch cleanup 对齐 — 实现规格 (Spec)

> 日期：2026-07-02（修订 1）
> 状态：草稿，待 review
> 范围：v1.5.0
> 承接：`docs/plans/2026-04-23-optimization-execution-plan.md` §5（2026-06-01）记录的"非回归条目"

## 0. 修订说明

初稿假设 `serveClean` 不存在 / 默认 false。实际代码（`cmd/gobin/commands/serve.go:119`）显示：

- `serveClean` 标志**已存在**，默认 `true`（一次性 build 走）
- `runServeWithOps` 在 watch 分支**显式** `runtime.cleanOutput = false`，理由是 "Wiping the output dir on every file save would throw away the manifest the watcher just primed"
- 这个显式覆盖就是"删文章留旧 HTML"的根源

**本 spec 修订方向**：保留 `--clean` 标志给一次性 build 用；watch 分支不再硬覆盖 `cleanOutput`；新增 `--no-clean-on-watch` 标志保留 v1.4.0 行为。

## 1. 问题

v1.4.0 `gobin serve --watch` 的 watcher 重建路径 `cleanOutput=false`，导致：

- 删文章后，磁盘上旧 `publishDir/2026/05/01-removed-post/index.html` 仍可访问 → 站外链入 200 但内容已删
- 删 / 改名静态资源后，磁盘上旧 `publishDir/assets/old-name.js` 仍可访问
- 与一次性 `gobin build` 的输出行为不一致（build 走 manifest stale cleanup；watch 不走）

进度记录里把它显式记为"非回归"，但用户大概率会撞上。

## 2. 目标

v1.5.0 把 `serve --watch` 的清理行为对齐到 `gobin build --clean=true`：

- 删除文章 → 旧 `index.html` 从 `publishDir` 移除
- 删除 / 重命名静态资源 → 旧文件从 `publishDir` 移除
- watch 模式不再有"删除文章留旧 HTML"的语义陷阱

## 3. 设计决策

| 决策 | 选择 | 理由 |
|------|------|------|
| 现有 `--clean` 标志 | 保留，仅影响一次性 build | v1.4.0 行为不变 |
| watch 分支的硬覆盖 | 移除 | 是 watch 路径下"不清孤儿"的源头 |
| 兜底标志 | 新增 `--no-clean-on-watch`（默认 false） | 给"在另一进程里维护 publishDir"的高级用户留 escape hatch |
| 实现位置 | `cmd/gobin/commands/serve.go` 的 watch 分支，去掉 `runtime.cleanOutput = false` | 单点改动 |
| partial rebuild 兼容性 | 删除事件走 `classifyChange` 归类 structural → 触发全量重建 + cleanOutput | 已存在的 partial rebuild 框架，无需新代码 |
| `--clean=false` + `--no-clean-on-watch=false` 同时设 | 一次性 build 不清；watch 也不清 | 与 v1.4.0 完全一致，escape hatch 完整 |
| 行为切换不破坏现有测试 | `serve_test.go` 中 `runtime.cleanOutput` 仍为 `false` 显式传入的路径不受影响 | 既有测试覆盖的是 no-clean 路径 |

## 4. 行为契约

### 4.1 默认 watch 行为

```bash
gobin serve --watch
# 内部: runtime.cleanOutput 跟随 serveClean 标志
# 默认 serveClean=true → 一次性 build 清空 + watcher 重建走 cleanOutput=true
```

- 编辑 post → partial rebuild（已存在）
- 删 post → 全量重建（structural 变更）+ `publishDir` 按 manifest 清掉孤儿
- 编辑 .css → 资源重写
- 删 .css → 资源删除（已存在 partial rebuild 路径）
- 编辑模板 → structural 变更 → 全量重建 + 清孤儿

### 4.2 `--no-clean-on-watch`

```bash
gobin serve --watch --no-clean-on-watch
# 内部: runtime.cleanOutput = false（强制覆盖）
```

恢复 v1.4.0 行为。

### 4.3 watch 下的全量重建触发条件（不变 + 明确化）

不变：
- `config.yaml` 变化
- `templates/` 或主题 `layouts/` 变化
- 短代码模板变化
- env hash 失配

明确化（删除事件归到 structural 走全量 + cleanOutput）：
- 任意 post 被删 / 重命名
- 任意静态资源被删 / 重命名
- `_posts/` / `_pages/` / `assets/` 目录被删

## 5. 不在范围

- 改一次性 build 的 `cleanOutput` 默认值（保持 v1.4.0 行为）
- 把 `cleanOutput` 暴露给模板上下文
- watch 模式下加 `--clean` / `--no-clean` 的细粒度配置（已经有 `--no-clean-on-watch` 兜底）

## 6. 风险

- 全量重建耗时会比 partial 慢；对开发体验是负面。`make benchmark` 已有 `BenchmarkBuildFull/posts=100` 约 18 ms 量级，可接受
- 已存在的"用户在 watch 模式下同时手动维护 publishDir"工作流会被打破；`--no-clean-on-watch` 标志兜底
- 端到端测试需要覆盖"删 post → 旧 HTML 消失"，在 macOS / Linux 上 fsnotify 行为略有差异，要测两边

## 7. 验证

```bash
# 单元测试：classifyChange 对 DELETE 事件归到 structural
go test ./cmd/gobin/commands -run 'TestClassifyChange_DeleteIsStructural'

# 端到端：example-site 跑 serve，模拟删除 post
# 1. gobin serve --watch 启动
# 2. rm _posts/2026-05-01-some.md
# 3. 等到 debounce + rebuild 完成
# 4. assert: publishDir 下对应 index.html 不存在
# 5. assert: 其它 post 的 index.html 仍存在

# 兼容路径：--no-clean-on-watch
# 重复上面步骤，assert: 旧 index.html 仍在
```
