# 多静态资源目录（staticDirs）使用指南

> Gobin v1.8.2 起支持把多个顶层目录一并复制进 `publishDir`。主要解决从 Jekyll 继承下来的站点结构：`img/`、`images/` 等目录希望保留其目录名输出，而不是被摊平到站点根。

## 1. 为什么需要

Jekyll 会把项目根下所有非 `_` 开头的顶层目录都复制到 `_site/`。Gobin 默认只复制 `staticDir`（默认 `assets/`）里的内容，且**摊平**到站点根（`assets/img/a.png` → `public/img/a.png`）。

如果你的旧博客在根目录有 `img/`、`images/` 这样的目录，帖子正文用 `../img/xxx.png` 引用它们，那么只用 `staticDir` 会丢失这些图片（`public/img/` 只有 22 个来自 `assets/img/`，根目录 `img/` 的 523 个文件全部缺失）。

## 2. 配置

```yaml
# config.yaml
staticDir: assets          # 主目录，内容摊平到站点根
staticDirs:
  - assets
  - img                    # 额外目录，输出到 public/img/...
  - images                 # 额外目录，输出到 public/images/...
```

行为：

- **第一个位置（或仅 `staticDir`）**：内容摊平到 `public/` 根（`assets/css/main.css` → `public/css/main.css`，与旧行为一致）。
- **后续位置**：保留目录名作为输出前缀（`img/2024-05-01/a.png` → `public/img/2024-05-01/a.png`，`images/RabbitMQ/...` → `public/images/RabbitMQ/...`）。

这样 `../img/cover.png` 与 `../images/Prometheus/alert/alert.png` 等相对引用在渲染后依然有效。

## 3. 与 `staticDir` 的关系

- 未配置 `staticDirs` 时，等价于 `staticDirs: [staticDir]`，行为与旧版本完全一致（向后兼容）。
- 建议把 `staticDir` 也放进 `staticDirs` 第一项，语义最清晰；但即使不写第一项，主目录仍会被复制。

## 4. 验证

构建后检查：

```bash
find public/img -type f | wc -l      # 应包含 img/ 下全部文件
find public/images -type f | wc -l   # 应包含 images/ 下全部文件
```

用脚本统计帖子里的相对图片引用是否都能在 `public/` 落地：

```bash
grep -rohE '../(img|images)/[^)"]*' _posts | sort -u
```

## 5. 排错

| 症状 | 可能原因 | 处理 |
|------|----------|------|
| `public/img/` 缺文件 | 根目录 `img/` 未加入 `staticDirs` | 把 `img` 加进 `staticDirs` |
| 图片被摊平到 `public/` 根而不是 `public/img/` | 把 `img` 放在了 `staticDirs` 第一项 | 把主目录 `assets` 放第一项，`img`/`images` 放后面 |
| 帖子里的 `../img/...` 渲染后 404 | `staticDirs` 未包含对应目录，或路径不符 | 确认目录名与输出前缀一致 |
