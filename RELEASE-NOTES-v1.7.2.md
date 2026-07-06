# Gobin v1.7.2 发布说明

## 发布日期 - 2026-07-06

Gobin v1.7.2 收口 image-pipeline spec §9 最后一项 deferred 验收项："WebP 转换正确，浏览器能加载"。接入 `github.com/HugoSmits86/nativewebp`（纯 Go，零 cgo）作为真实 WebP 编码后端：新增 `WebPExecutor`（VP8L lossless 编码 + x/image/webp 解码），`NewDefaultExecutor` 改用 `WebPExecutor`，image pipeline 默认 webp-capable。`config.assets.images.formats: ["webp"]` 现在产出浏览器可加载的真实 WebP 字节，不再是 v1.7.0/v1.7.1 的 passthrough 占位。

---

## 亮点

- **WebP 真实编码**：`WebPExecutor` 基于 `nativewebp`（纯 Go，零 cgo），VP8L lossless 编码 + `x/image/webp` 解码。`formats: ["webp"]` 产出 RIFF/WEBP 签名的真实字节，浏览器可直接加载。
- **默认 executor 升级**：`NewDefaultExecutor()` 返回 `WebPExecutor`（`StdlibExecutor` 的超集），image pipeline 默认 webp-capable，无需额外配置。
- **混合 srcset**：单 executor 同时产出 webp + jpg（`<source type=image/webp>` + `<img>` JPEG fallback），渐进增强一步到位。
- **完全向后兼容**：config 默认 `formats: ["jpg","png"]` 不变，未显式加 `"webp"` 的站点产物与 v1.7.1 字节级一致；公开 API 仅新增类型 + 函数。

---

## 升级方式

```bash
go install github.com/mengbin92/gobin/cmd/gobin@v1.7.2
```

或从 GitHub Releases 下载对应平台的压缩包，并使用 `SHA256SUMS` 校验。

Docker 用户可以使用：

```bash
docker pull docker.io/mengbin92/gobin:v1.7.2
```

---

## WebP 真实编码

### 1. 解决什么问题

v1.7.0 / v1.7.1 的 stdlib executor 只能编码 jpg/png；`formats: ["webp"]` 走 passthrough（不重编码，只复制原图字节到目标文件名），体积不会下降——这正是 `docs/guides/image-pipeline.md` 排错表列出的已知坑。spec §9 验收清单里"WebP 转换正确，浏览器能加载"一直标记为 Deferred to v1.7.2。

### 2. 怎么用

config 里把 `"webp"` 加进 `formats`：

```yaml
assets:
  images:
    enabled: true
    srcset: [480, 800, 1200]
    formats: ["webp", "jpg"]   # webp + jpg fallback
    quality: 80
```

build 后产物会多出 `cover-480w.webp` / `cover-800w.webp` 等真实 WebP 变体，postprocess 把 `<img>` 改写为 `<picture><source type="image/webp" srcset="..."> <img srcset="...">`。

### 3. 编码特性

- **VP8L lossless**：`nativewebp` 只做 lossless WebP 编码。`quality` 参数映射到 `CompressionLevel`（0=BestSpeed … 6=BestCompression），更高的 quality = 更高压缩努力。
- **alpha 原生支持**：PNG alpha → WebP round-trip 透明度不丢（`TestTransform_WebPAlphaPreserved` 覆盖）。
- **lossy WebP 留 v1.8+**：lossless 体积偏大于 lossy（q=80 lossy 通常更小），但作为 v1.7.2 首个真实 WebP 后端可接受；spec §9 只要求"浏览器能加载"。

### 4. 测试覆盖（spec §9）

| 测试 | 覆盖 |
|---|---|
| `TestTransform_WebPRealEncoding` | RIFF/WEBP 签名 + round-trip 解码 + 尺寸 |
| `TestTransform_WebPMixedSrcset` | webp + jpg 混合 srcset，两者均可解码 |
| `TestTransform_WebPFromWebPSource` | .webp 源解码 + 重编码为更小尺寸 |
| `TestTransform_WebPAlphaPreserved` | PNG alpha → WebP round-trip 透明度不丢 |
| `TestNewDefaultExecutorIsWebPCapable` | 默认 executor 真实编码 WebP |
| `TestTransform_WebPViaExecutorInterface` | Executor 接口契约（mock，保留） |

`TestTransform_InvalidFormatReturnsError` 调整为用 `StdlibExecutor` + `"bogus"` 格式（默认 executor 现已支持 webp）。

---

## 库 API 变化

```go
// 新增（v1.7.2）
type WebPExecutor struct { StdlibExecutor }
func NewWebPExecutor() *WebPExecutor
func NewDefaultExecutor() Executor  // 返回 WebPExecutor

// 不变
type StdlibExecutor struct{}
func NewStdlibExecutor() *StdlibExecutor  // 仍是零第三方依赖后端（jpg/png only）
func Transform(src []byte, sourceExt string, opts TransformOptions, exec Executor) ([]Variant, error)
```

新增直接依赖：`github.com/HugoSmits86/nativewebp v1.3.0`（传递依赖 `golang.org/x/image v0.24.0`），纯 Go，零 cgo。

---

## 兼容性说明

- 公开 API 100% 向后兼容（仅新增类型 + 函数；现有签名不变）。
- `assets.images.enabled: false`（默认）：行为与 v1.7.1 / v1.7.0 / v1.6 字节级一致。
- config 默认 `formats: ["jpg", "png"]` 保持不变（与 v1.7.1 行为一致）；用户显式加 `"webp"` 才产出 WebP 变体。默认 executor 虽已 webp-capable，但不改 config 默认值，避免未声明用户意外改变产物。
- 增量构建 manifest 不受影响（`options_hash` 覆盖 formats，加/去 webp 会触发该 source 重转）。
- v1.5 资产指纹管线不受影响。

---

## 已知限制 / Deferred

- **VP8L lossless only**：体积偏大于 lossy WebP；lossy WebP 需 libvips 或 disintegration/imaging 的 lossy 路径，列入 v1.8+。
- AVIF / LQIP / blurhash / EXIF 保留 / CDN 预热：列入 v1.8+ 候选，未做。

---

## 验证

发布前建议执行：

```bash
make test
go test -race ./internal/parser/... ./internal/generator/... ./internal/imaging/... ./cmd/gobin/commands/...
go vet ./...
gofmt -l $(find . -name '*.go' -not -path './website/*' -not -path './public/*' -not -path './example-site/*')
go mod tidy
make release-local
```
