# Gobin v1.8.2 发布说明

## 发布日期 - 2026-08-11

Gobin v1.8.2 新增**多静态资源目录（staticDirs）**支持，解决从 Jekyll 继承的站点结构（根目录 `img/`、`images/` 等）在迁移到 Gobin 后图片丢失的问题。

## 新增内容

- 新增 `staticDirs` 配置项：可声明多个静态目录，一并复制进 `publishDir`。
  - 第一项（或仅 `staticDir`）内容摊平到站点根（`assets/css/main.css` → `public/css/main.css`）。
  - 后续目录保留目录名作为输出前缀（`img/2024-05-01/a.png` → `public/img/2024-05-01/a.png`）。
- `serve --watch` 监听所有 `staticDirs`，任一目录变化都会按静态资源变更触发增量复制。
- 配置校验：每个 `staticDirs` 目录不得与 `publishDir` 重叠。

## 迁移自 Jekyll 的配置示例

```yaml
staticDir: assets
staticDirs:
  - assets
  - img
  - images
```

## 兼容性

- 未配置 `staticDirs` 时行为与 v1.8.1 完全一致（等价于 `staticDirs: [staticDir]`）。
- 无模板语法变更。
- 无公开 Go API 变更（新增字段 `Config.StaticDirs`）。

## 验证

```bash
go test ./...
go vet ./...
```
