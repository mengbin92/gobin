# Gobin v1.7.2 更新日志

## 发布日期 - 2026-07-06

Gobin v1.7.2 收口 image-pipeline spec §9 最后一项 deferred 验收项："WebP 转换正确，浏览器能加载"。接入 `github.com/HugoSmits86/nativewebp`（纯 Go，零 cgo）作为真实 WebP 编码后端：新增 `WebPExecutor`（VP8L lossless 编码 + x/image/webp 解码），`NewDefaultExecutor` 改用 `WebPExecutor`，image pipeline 默认 webp-capable。`config.assets.images.formats: ["webp"]` 现在产出浏览器可加载的真实 WebP 字节，不再是 v1.7.0/v1.7.1 的 passthrough 占位。本次发布保持配置、模板、CLI 入口、增量构建、并行构建、shortcodes、serve partial rebuild 既有行为完全向后兼容。

---

## 新增功能

### WebP 真实编码（v1.7.2）

- 新增 `internal/imaging/webp.go`：`WebPExecutor`（嵌入 `StdlibExecutor`，覆写 `Decode` / `Encode`）。
  - `Decode`：先试 stdlib JPEG/PNG，再 fallback 到 `nativewebp.Decode`（包装 `golang.org/x/image/webp`，支持 lossy + lossless）。
  - `Encode`：`webp` 走 `nativewebp.Encode`（VP8L lossless），`quality` 映射到 `CompressionLevel`（0=BestSpeed … 6=BestCompression）；`jpg`/`png` 复用 stdlib encoder，支持单 executor 产出混合格式 srcset。
- 新增 `NewDefaultExecutor() Executor`：返回 `WebPExecutor`，作为 image pipeline 默认 executor。v1.7.2 起 `formats: ["webp"]` 开箱即用。
- `internal/generator/image_pipeline.go`：`NewStdlibExecutor()` → `NewDefaultExecutor()`，唯一一处接线改动。

### 测试（spec §9 收口）

- `TestTransform_WebPViaExecutorInterface` 保留为 Executor 接口契约测试（mock，验证 Encode 调用 + 字节到达 variant）。
- 新增 `TestTransform_WebPRealEncoding`：spec §9 验收项主测试——RIFF/WEBP 签名 + round-trip 解码 + 尺寸。
- 新增 `TestTransform_WebPMixedSrcset`：单 source 产出 webp + jpg 混合 srcset，两者均可解码。
- 新增 `TestTransform_WebPFromWebPSource`：.webp 源被 `WebPExecutor.Decode` 解码并重编码为更小尺寸。
- 新增 `TestTransform_WebPAlphaPreserved`：PNG alpha → WebP round-trip 透明度不丢（VP8L lossless 原生支持 alpha）。
- 新增 `TestNewDefaultExecutorIsWebPCapable`：默认 executor 真实编码 WebP（RIFF 签名）。
- `TestTransform_InvalidFormatReturnsError` 调整：改用 `StdlibExecutor` + `"bogus"` 格式（因为默认 executor 现已支持 webp，旧用例用 webp 会成功而非报错）。

---

## 库 API 变化

- 新增 `imaging.WebPExecutor` 结构体（嵌入 `StdlibExecutor`）。
- 新增 `imaging.NewWebPExecutor() *WebPExecutor`。
- 新增 `imaging.NewDefaultExecutor() Executor`（返回 `WebPExecutor`）。
- `StdlibExecutor` / `NewStdlibExecutor` / `Transform` / `TransformOptions` / `Variant` 签名全部不变。`StdlibExecutor` 仍是零第三方依赖后端（仅 jpg/png），可用于不需要 WebP 的场景。
- 新增直接依赖：`github.com/HugoSmits86/nativewebp v1.3.0`（传递依赖 `golang.org/x/image v0.24.0`），纯 Go，零 cgo。

---

## 兼容性

- 公开 API 100% 向后兼容（仅新增类型 + 函数；现有签名不变）。
- `assets.images.enabled: false`（默认）：行为与 v1.7.1 / v1.7.0 / v1.6 字节级一致（image pipeline 不运行）。
- config 默认 `formats: ["jpg", "png"]` 保持不变（与 v1.7.1 行为一致）；用户显式加 `"webp"` 才产出 WebP 变体。默认 executor 虽已 webp-capable，但不改 config 默认值，避免未声明用户意外改变产物。
- 增量构建 manifest 不受影响（`options_hash` 覆盖 formats，加/去 webp 会触发该 source 重转）。
- v1.5 资产指纹管线不受影响。

---

## 已知限制 / Deferred

- **VP8L lossless only**：`nativewebp` 只做 lossless WebP 编码，体积偏大于 lossy WebP（q=80 lossy 通常更小）。lossy WebP 需 libvips 或 disintegration/imaging 的 lossy 路径，列入 v1.8+。
- AVIF / LQIP / blurhash / EXIF 保留 / CDN 预热：列入 v1.8+ 候选，未做。

---

## 验证

发布前执行：

```bash
go test ./...
go test -race ./internal/parser/... ./internal/generator/... ./internal/imaging/... ./cmd/gobin/commands/...
go vet ./...
gofmt -l internal/ cmd/
go mod tidy
```

（已通过。）
