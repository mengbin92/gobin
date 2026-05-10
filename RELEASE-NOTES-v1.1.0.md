# Gobin v1.1.0 发布说明

## 发布日期 - 2026-05-10

Gobin v1.1.0 聚焦生成器演进、开发体验和发布流程硬化。这个版本向后兼容 v1.0.0。

---

## 亮点

- **更清晰的生成器结构**：内容规划、页面规划和 artifact 执行边界进一步拆分。
- **更好的错误诊断**：模板渲染失败时会包含输出路径、页面标题和模板候选信息。
- **更实用的增量构建基础**：未变化页面跳过重写，并在 CLI 输出中展示 rendered/skipped。
- **资源缓存友好**：模板可通过 `assetURL` 自动追加内容 hash 查询参数。
- **开发体验提升**：`gobin serve` 在 watch 模式下支持 LiveReload。
- **发布更可靠**：Release 产物包含压缩包和 `SHA256SUMS`。

---

## 升级方式

```bash
go install github.com/mengbin92/gobin/cmd/gobin@v1.1.0
```

或从 GitHub Releases 下载对应平台的压缩包，并用 `SHA256SUMS` 校验。

---

## 新增命令行为

### 构建统计

`gobin build` 和 `gobin serve` 现在会输出页面与静态资源统计：

```text
Pages: rendered 12, skipped 4
Static assets: copied 2, skipped 10, deleted 0
```

### LiveReload

默认 watch 模式会启用 LiveReload：

```bash
gobin serve
```

关闭 LiveReload：

```bash
gobin serve --live-reload=false
```

---

## 模板更新

新增 `assetURL` 模板函数：

```html
<link rel="stylesheet" href="{{ assetURL "/css/main.css" }}">
```

本地资源会输出为：

```html
<link rel="stylesheet" href="/css/main.css?v=<content-hash>">
```

---

## 兼容性说明

- 配置文件不需要迁移。
- 现有内容、模板和主题可以继续使用。
- `assetURL` 是可选新增能力。
- LiveReload 只影响开发服务器响应，不会写入生成后的 HTML 文件。

---

## 验证

发布前验证项：

```bash
go test ./...
make lint
git diff --check
```
