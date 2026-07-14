# serve watch 行为指南

> Gobin v1.5.0 起把 `serve --watch` 的清理行为对齐到 `gobin build --clean=true`。本指南说明 v1.5.0 行为、escape hatch、和 watch 下的全量重建触发条件。

## 1. v1.4.0 的痛点

v1.4.0 `gobin serve --watch` 的 watcher 重建路径 `cleanOutput=false`，导致：

- 删文章后，磁盘上旧 `publishDir/2026/05/01-removed-post/index.html` 仍可访问 → 站外链入 200 但内容已删
- 删 / 改名静态资源后，磁盘上旧 `publishDir/assets/old-name.js` 仍可访问
- 与一次性 `gobin build` 的输出行为不一致

进度记录里把它显式记为"非回归"。

## 2. v1.5.0 默认行为

```bash
gobin serve --watch
# 内部: runtime.cleanOutput = serveClean（默认 true）
# → 一次性 build 清空 + watcher 重建走 cleanOutput=true
```

| 事件 | v1.4.0 | v1.5.0 |
|------|--------|--------|
| 编辑 post | partial rebuild | partial rebuild |
| **删 post** | partial rebuild，旧 HTML 留下 | 全量重建 + manifest 清孤儿 |
| 编辑 .css | 资源重写 | 资源重写 |
| **删 .css** | 资源重写，旧文件留下 | 全量重建 + 资源清孤儿 |
| 编辑模板 | structural → 全量 | structural → 全量 |

`cleanOutput=true` 触发 `internal/generator/build_manifest.go` 的 stale cleanup：上一次构建 manifest 里记录过、但本次不再出现的产物会从 `publishDir` 删除。

## 3. Escape hatch：`--no-clean-on-watch`

需要恢复 v1.4.0 行为（比如在另一进程里维护 `publishDir`，或 watch 模式跑在慢盘上）：

```bash
gobin serve --watch --no-clean-on-watch
```

行为：watcher 重建永远 `cleanOutput=false`，stale 不清。`--clean` 标志只影响一次性初始 build，不影响 watcher 重建。

## 4. watch 下的全量重建触发条件

只要任一条件成立，watcher 会从 partial rebuild 退回全量 + cleanOutput：

- `config.yaml` / `config.yml` / `_config.yml` / `_config.yaml` 变化
- `templates/`、`_layouts/`、`_includes/` 或主题 `<theme>/` 下任何文件变化
- 短代码模板变化
- 任何 `.md` / `.markdown` 文件被删 / 重命名
- 任何静态资源被删 / 重命名
- `_posts/` / `_pages/` / `assets/` 目录被删

编辑（Create / Write）事件走 partial rebuild，删除（Remove）事件走全量。

## 5. 与 `--incremental` 的关系

- `gobin build --incremental` 必须搭配 `--clean=false`，`build` 命令会显式拒绝 `--incremental --clean=true`
- `gobin serve --watch` 的 `cleanOutput` 与 incremental 不互相约束：watcher 内部用 `serveBuilder.rebuildResult`，它检测到 `cleanOutput=true` 时**自动**把 `Incremental` 关掉（clean 会清 manifest，再 incremental 没意义）

## 6. 排错

| 症状 | 可能原因 | 处理 |
|------|----------|------|
| 编辑 post 后站点没更新 | incremental 把它判为 unchanged | `gobin build --clean=true` 重置；或加 `--no-clean-on-watch` + `gobin build` 重建 |
| 删 post 后旧 HTML 还在 | `serve --no-clean-on-watch` | 去掉该 flag |
| watch 模式报 `--incremental cannot be combined with --clean=true` | 不太可能：watcher 内部自动处理 | 提 issue |
| 删除文章后 partial rebuild 跳过了 | 实际是结构性变更 → 走了全量 + cleanOutput | 正常行为；日志里看 `Full reload` |
| watch 下 `make benchmark` 数字差 | watch 退全量是预期内 | 与一次性 build 对比才有意义 |
