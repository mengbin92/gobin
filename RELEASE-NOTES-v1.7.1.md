# Gobin v1.7.1 发布说明

## 发布日期 - 2026-07-04

Gobin v1.7.1 是 v1.7.0 图片优化管线的 patch 版本，收口 `docs/superpowers/specs/2026-07-04-image-pipeline-design.md` 的 §7/§8/§9：把 v1.7.0 留下的 image pipeline 增量构建 TODO 落地，补齐 spec §8 列出的全部单元 / 集成测试，并对照 spec §9 验收清单逐项验证。**真实 22 张图未变化时增量构建从 ~475ms 降到接近零开销**（只做 SHA-256 + 文件 stat），单图源修改时只重转该 source 的变体，其余 source 全部跳过。

---

## 亮点

- **image pipeline 增量构建**：`runImagePipeline` 在每个 source 写入前比对 `source_hash`（源文件 SHA-256）+ `options_hash`（`srcset` / `formats` / `sizes` / `quality` 拼接的 SHA-256）+ 变体文件在盘状态。命中则跳过并累计 `ImageStats.Skipped`，不再每次全量重转。
- **manifest 向前兼容**：`.gobin-images.json` 每条 entry 新增 `source_hash` / `options_hash` 字段（omitempty）。v1.7.0 老_manifest 缺失字段时按空串处理 → 走一次全量重转，之后恢复正常 skip，无破坏。
- **spec §8/§9 测试收口**：新增 5 条 imaging 单测 + 6 条 image pipeline 集成测试，覆盖 disabled 字节一致 / enabled `<picture>` / front matter cover / 增量（不变 / 删变体 / 改源）/ 单图失败兜底。
- **完全向后兼容**：`assets.images.enabled: false`（默认）行为与 v1.7.0 / v1.6 字节级一致；公开 API 仅新增 JSON 字段 + `Skipped` 计数，签名全部不变。

---

## 升级方式

```bash
go install github.com/mengbin92/gobin/cmd/gobin@v1.7.1
```

或从 GitHub Releases 下载对应平台的压缩包，并使用 `SHA256SUMS` 校验。

Docker 用户可以使用：

```bash
docker pull docker.io/mengbin92/gobin:v1.7.1
```

---

## image pipeline 增量构建

### 1. 解决什么问题

v1.7.0 的 image pipeline 每次构建都重新生成所有变体（22 张图 × 3 宽度 × 2 格式 = 132 个 variants，~475ms），即便图片与配置都未变化。spec §2 明确要求"基于源文件内容 hash 跳过"，但 v1.7.0 作为 baseline 先发版，留作下个 patch。

### 2. 怎么用

无需任何配置改动。开启 `assets.images.enabled: true` 后：

- **第一次构建**：全量转换所有 source，写 `.gobin-images.json`（含每条 `source_hash` / `options_hash`）。
- **第二次构建（无变化）**：所有 source 命中 skip，累计 `ImageStats.Skipped`，接近零开销。
- **改了某张图**：只重转该 source 的变体，其余 source 仍走 skip。
- **手工删了某个变体文件**（如 `cover-480w.jpg`）：触发该 source 的重转，其余 sources 仍 skip。
- **改了 `srcset` / `formats` / `sizes` / `quality`**：`options_hash` 变化 → 所有 source 重转。

### 3. manifest 兼容

`.gobin-images.json` 每条 entry：

```json
{
  "source": "...",
  "variants": [...],
  "source_hash": "<sha256 of source file>",
  "options_hash": "<sha256 of srcset+formats+sizes+quality>"
}
```

读端忽略未知字段，v1.7.0 写出的旧 manifest 仍能正确解析（两个 hash 为空 → 第一次构建走全量，之后恢复正常 skip）。

### 4. 测试覆盖（spec §8）

**单元测试**（`internal/imaging/imaging_test.go`，spec §8.1）：

| Spec 名称 | 实际测试名 | 覆盖 |
|---|---|---|
| `TestTransform_EmptyFormats` | `TestTransform_EmptyFormatsIsSingleSourceSize` | 空 options → 单条原尺寸 passthrough |
| `TestTransform_InvalidFormat` | `TestTransform_InvalidFormatReturnsError` | 未知 format → executor 报 error |
| `TestTransform_Quality` | `TestTransform_QualityDifference` | q=30 vs q=95 输出大小可观测差异 |
| `TestTransform_PNG` | `TestTransform_PNGPreservesAlphaChannel` | PNG alpha round-trip 不丢 |
| `TestTransform_WebP` | `TestTransform_WebPViaExecutorInterface` | mock Executor 验证接口契约（真实 WebP 编码 deferred 到 v1.7.2） |

**集成测试**（`internal/generator/image_pipeline_test.go`，spec §8.2 / §9）：

| Spec 名称 | 实际测试名 | 覆盖 |
|---|---|---|
| `TestImagePipeline_DisabledIsByteIdentical` | 同名 | 关闭时零写入，字节级一致 |
| `TestImagePipeline_EnabledGeneratesPictureTags` | 同名 | manifest 写入 + 变体在盘 + postprocess 出 `<picture>` |
| `TestImagePipeline_FrontMatterCover` | 同名 | front matter cover 触发整条链路 |
| `TestImagePipeline_Incremental` | 同名 | 3 轮：cold / 全部 skip / 删变体后单源重转 |
| `TestImagePipeline_SourceChangeTriggersRetransform` | 同名 | 源变 → 该源重转，其余 skip |
| `TestImagePipeline_PerSourceFailureDoesNotAbort` | 同名 | 单图失败不阻断 + 兜底拷贝原图 |

---

## 库 API 变化

- `imaging.Stats.Skipped` 字段从无到有：累计"因源 + 配置 + 变体未变而跳过的 source 数"，与 `Sources` / `Variants` / `Errors` 并列。
- `imageManifestEntry` JSON 多了 `source_hash` / `options_hash` 两个字段（omitempty）。读端忽略未知字段，v1.7.0 写出的 manifest 仍能正确解析为"两个 hash 为空"→ 第一次构建走全量。
- 无新导出函数 / 无新导出类型；现有签名全部不变。

---

## 兼容性说明

- 公开 API 100% 向后兼容（仅新增 JSON 字段 + `Skipped` 计数）。
- `assets.images.enabled: false`（默认）：行为与 v1.7.0 / v1.6 字节级一致（`TestImagePipeline_DisabledIsByteIdentical` 覆盖）。
- v1.5 资产指纹管线不受影响。
- v1.7.0 老_manifest 自动兼容（缺 hash → 走全量一次，之后再 skip）。

---

## 已知限制 / Deferred

- **WebP 真实编码 deferred 到 v1.7.2**：stdlib executor 主动编码 jpg/png；其它格式（含 webp）走 passthrough。`TestTransform_WebPViaExecutorInterface` 已验证 Executor 接口契约，真实 WebP 编码待 v1.7.2 接入 `disintegration/imaging` 或 `github.com/HugoSmits86/nativewebp`。
- AVIF / LQIP / CDN 预热 / EXIF 保留：列入 v1.7.2+ 候选，未做。

---

## 验证

发布前建议执行：

```bash
make test
go test -race ./internal/parser/... ./internal/generator/... ./internal/imaging/... ./cmd/gobin/commands/...
go vet ./...
gofmt -l $(find . -name '*.go' -not -path './website/*' -not -path './public/*' -not -path './example-site/*')
make release-local
```
