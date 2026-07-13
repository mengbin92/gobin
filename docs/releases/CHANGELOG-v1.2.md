# Gobin v1.2.0 更新日志

## 发布日期 - 2026-05-22

Gobin v1.2.0 是一次向后兼容的功能版本，重点提升内容创作、构建校验、增量构建和容器化发布体验。

---

## 新增功能

### 内容创作

- 新增 `gobin new post <title>`，按 `contentDir` 创建带 Front Matter 的文章草稿。
- 新增 `gobin new page <title>`，按 `pageDir` 创建带 Front Matter 的页面草稿。
- `gobin new post` 支持 `--date YYYY-MM-DD` 和 `--force`。

### 站点校验

- 新增 `gobin check`，在不写入输出目录的情况下检查配置、内容、模板和 permalink 冲突。
- `gobin check --drafts` 可将草稿内容纳入冲突检查，适合 CI 和发布前验证。

### 资源指纹

- `assets.fingerprint.strategy` 新增 `filename` 策略，支持按内容 hash 输出 `name.<hash>.ext`。
- manifest 记录实际落盘路径，内容变化或源文件删除时会清理旧 fingerprint 文件。

### 增量构建

- 新增 `<publishDir>/.gobin-build.json` 构建 manifest。
- `gobin build --incremental --clean=false` 可跳过未变化的单内容页、列表页、taxonomy 页和站点级聚合产物。
- `gobin serve` watcher 保存时自动使用增量重建路径。

### Docker 发布

- Dockerfile 支持 BuildKit 多架构构建。
- Release workflow 会发布 `docker.io/mengbin92/gobin:<tag>` 和 `docker.io/mengbin92/gobin:latest`。
- Docker Hub 镜像支持 `linux/amd64` 和 `linux/arm64`。

---

## 改进

- 本地和 GitHub Actions release 产物统一使用 `gobin-vX.Y.Z-平台-架构` 命名。
- Docker 镜像构建注入版本号、commit 和构建时间。
- README 和发布指南补充 Docker 镜像使用方式与发布检查项。

---

## 兼容性

- 本版本保持配置和内容结构向后兼容。
- 默认构建仍会清理输出目录；只有显式使用 `--incremental --clean=false` 时才启用增量跳过。
- 默认资源 URL 版本化行为保持兼容，文件名级指纹需要显式配置。

---

## 验证

发布前执行：

```bash
go test ./...
make lint
git diff --check
```
