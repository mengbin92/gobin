# Gobin v1.2.0 发布说明

## 发布日期 - 2026-05-22

Gobin v1.2.0 是一次向后兼容的功能版本，重点完善内容创作命令、站点校验、资源缓存策略、增量构建和容器化发布。

---

## 亮点

- **新增内容脚手架**：`gobin new post|page` 可快速创建带 Front Matter 的文章或页面草稿。
- **新增站点校验**：`gobin check` 在不写入输出目录的情况下校验配置、内容解析、模板加载和 permalink 冲突。
- **文件名级资源指纹**：`assets.fingerprint.strategy: filename` 支持生成 `name.<hash>.ext` 形式的静态资源。
- **更完整的增量构建**：`gobin build --incremental --clean=false` 使用 manifest 跳过未变化的单页、列表、taxonomy、feed、sitemap、search、alias 和 robots 产物。
- **更快的开发服务器重建**：`gobin serve` watcher 保存时自动走增量重建路径。
- **Docker Hub 多架构镜像**：发布 `docker.io/mengbin92/gobin:v1.2.0` 和 `docker.io/mengbin92/gobin:latest`，支持 `linux/amd64` 和 `linux/arm64`。

---

## 升级方式

```bash
go install github.com/mengbin92/gobin/cmd/gobin@v1.2.0
```

或从 GitHub Releases 下载对应平台的压缩包，并使用 `SHA256SUMS` 校验。

Docker 用户可以使用：

```bash
docker pull docker.io/mengbin92/gobin:v1.2.0
```

---

## 新增命令

### 创建内容

```bash
gobin new post "我的第一篇文章"
gobin new post --date 2026-05-01 "Release notes"
gobin new page "About"
```

### 校验站点

```bash
gobin check
gobin check --drafts
```

`check` 会验证配置加载、Markdown/Front Matter 解析、主题模板加载，以及多个页面是否会写入同一个输出路径。

---

## 增量构建

```bash
gobin build --incremental --clean=false
```

增量构建会在 `<publishDir>/.gobin-build.json` 中记录构建指纹。无变化时会跳过已稳定的页面和站点级产物；清理构建可继续使用默认的 `gobin build`。

---

## Docker 镜像

Git tag 发布时会同时构建并推送 Docker Hub 镜像：

- `docker.io/mengbin92/gobin:v1.2.0`
- `docker.io/mengbin92/gobin:latest`

镜像支持：

- `linux/amd64`
- `linux/arm64`

运行示例：

```bash
docker run --rm -p 8080:8080 \
  -e GOBIN_AUTO_INIT=true \
  -v "$PWD:/site" \
  docker.io/mengbin92/gobin:v1.2.0
```

---

## 兼容性说明

- 本版本保持配置、内容结构和模板接口向后兼容。
- `gobin new`、`gobin check`、`--incremental` 和文件名级资源指纹均为新增能力。
- 默认资源指纹策略仍保持兼容行为；只有显式配置 `filename` 时才改写文件名。

---

## 验证

发布前建议执行：

```bash
go test ./...
make lint
git diff --check
```
