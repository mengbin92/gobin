# Gobin v1.5.0 发布说明

## 发布日期 - 2026-07-02

Gobin v1.5.0 是一次面向"构建产物与运行时一致性"的功能版本。本次发布在 v1.4.0 的基础上收口了三件事：资源管线的 HTML 引用改写与资产 hash 验证、`serve` watcher 与 `build --clean=true` 的清理对齐、以及统一日志系统的 `config.yaml` 段与环境变量覆盖。

---

## 亮点

- **资源管线收口**：构建产物走完 minify 后会再扫一遍 `publishDir` 下的 `.html`，把 `<link href>` / `<script src>` / `<img src>` 重写为指纹化后的路径；新增 `gobin check --assets` 验证磁盘 hash 与 manifest 是否一致，适合作为 CI 门禁；默认指纹扩展名新增 `.mjs`、`.avif`。
- **`serve` watcher 清理对齐**：`gobin serve --watch` 的重建路径移除 v1.4.0 的 `cleanOutput=false` 硬编码，默认走 `serveClean=true`，与一次性 `gobin build` 一致；新增 `--no-clean-on-watch` 逃生口。
- **统一日志的 `config.yaml` + env 覆盖**：v1.5.0 把 v1.4.0 明确划入 Phase 4 的"配置段接线"做完，新增顶层 `logging` 段与 `GOBIN_LOG_LEVEL` / `GOBIN_LOG_FORMAT` / `GOBIN_LOG_FILE` 环境变量；优先级 `flag > env > config > default`；配置源（config / env）的拼写错误会在启动期 fail，不再悄悄回退。

---

## 升级方式

```bash
go install github.com/mengbin92/gobin/cmd/gobin@v1.5.0
```

或从 GitHub Releases 下载对应平台的压缩包，并使用 `SHA256SUMS` 校验。

Docker 用户可以使用：

```bash
docker pull docker.io/mengbin92/gobin:v1.5.0
```

---

## 资源管线：HTML 引用改写 + 资产 hash 验证

### 1. HTML 引用改写

当 `assets.fingerprint.strategy = filename` 时，Gobin 在 minify 之后会再扫一遍 `publishDir` 下的 `.html`，把里面的 `<link href>` / `<script src>` / `<img src>` 改写为指纹化后的路径。模板里写死的 `href="/css/site.css"` 现在自动指向 `css/site.<hash>.css`，无需在模板里写 helper。

| 场景 | 改写? |
|------|------|
| `<link href="/css/site.css">` | ✓ |
| `<script src="/js/app.js">` | ✓ |
| `<img src="/img/cover.png">` | ✓ |
| `<link HREF="...">`（大写属性名） | ✓（大小写不敏感） |
| `<a href="/some-page/">` | ✗（避免误改站外 / 站内跳转） |
| `<link rel="alternate" href="...">` | ✗（feed/atom 引用，不需要 cache-bust） |
| `<form action="...">` | ✗ |
| `href="https://..."` / `href="//cdn..."` | ✗（跨站） |

### 2. `gobin check --assets`

发布前的资产 hash 一致性检查。它读 `.gobin-assets.json` manifest，验证每个指纹化文件的磁盘内容 hash 与其文件名中嵌入的 hash 一致。

```bash
gobin build --minify
gobin check --assets
# [OK] verified 14 fingerprinted asset(s) in public
# 退出码 0 = 全部一致，1 = 有 mismatch 或其它错误
```

典型 CI 用法：

```yaml
- run: gobin build --minify
- run: gobin check --assets
```

### 3. 默认指纹扩展名扩充

`DefaultAssetsFingerprintExtensions` 在 v1.5.0 加入 `.mjs`、`.avif`；不写 `assets.fingerprint.extensions` 即可获得完整列表。

### 4. 趋势化基准门禁（顺带提一句）

`scripts/check-benchmark.sh` 在 v1.5.0 同时检查绝对上限与相对回归：

- 绝对上限（不变）：5 个核心 benchmark 的 order-of-magnitude 阈值。
- 相对回归（新增）：当前 `ns/op` > `1.5x` 上次 `HEAD:benchmark-results.txt` → 红；> `1.2x` → 黄。

---

## `serve` watch 行为

### v1.5.0 默认行为

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

### Escape hatch：`--no-clean-on-watch`

```bash
gobin serve --watch --no-clean-on-watch
```

适用场景：另一进程维护 `publishDir`、watch 模式跑在慢盘上、或需要在 watch 期间手工放一些"不被 Gobin 看见"的产物。

---

## 统一日志系统的 `config.yaml` 段与 env 覆盖

v1.4.0 的统一日志系统规范把 `config.yaml` 接线明确划为 Phase 4，v1.5.0 把它做完。

### 1. 优先级

```
CLI flag > env var > config.yaml > 默认值
```

| 来源 | 覆盖对象 |
|------|----------|
| `--verbose` / `--log-format=...` / `--log-file=...` | 一切 |
| `GOBIN_LOG_LEVEL` / `GOBIN_LOG_FORMAT` / `GOBIN_LOG_FILE` | config 段 |
| `config.yaml` 的 `logging` 段 | 默认值 |
| 缺省 | level=info, format=text, output=stderr |

### 2. `config.yaml` 配置段

```yaml
logging:
  level: info       # debug | info | warn | error
  format: text      # text | json
  file: ""          # 空 → stderr；非空 → 追加写
```

字段含义同 CLI flag。配置段的值会被 CLI flag / env var 覆盖。

### 3. 校验差异

| 来源 | level 非法 | format 非法 | file 不可写 |
|------|-----------|-------------|-------------|
| config.yaml 段 | **启动报错** | **启动报错** | **启动报错** |
| env var（被 config 解析层读到） | **启动报错** | **启动报错** | **启动报错** |
| CLI flag | 容错回退 info | 容错回退 text | 容错回退 stderr 并 WARN |

> 思路：commit 进仓库的 `config.yaml` 是站点的"硬规则"，拼错应该 fail；CLI flag 是临时调试用，回退更友好。

### 4. 输出通道回顾（与 v1.4.0 一致）

| 输出目标 | 内容 |
| --- | --- |
| stdout | 面向用户的信息（版本号、统计、`[OK]`/`[FAIL]`、成功提示） |
| stderr / 文件（slog） | 结构化诊断日志，带 `level`、`msg`、`component` 等键 |

---

## Docker 镜像

Git tag 发布时会同时构建并推送 Docker Hub 镜像：

- `docker.io/mengbin92/gobin:v1.5.0`
- `docker.io/mengbin92/gobin:latest`

镜像支持：

- `linux/amd64`
- `linux/arm64`

运行示例：

```bash
docker run --rm -p 8080:8080 \
  -e GOBIN_AUTO_INIT=true \
  -v "$PWD:/site" \
  docker.io/mengbin92/gobin:v1.5.0
```

---

## 兼容性说明

- 本版本保持配置（除新增可选 `logging` 段外）、内容结构、模板接口、CLI 入口、并行构建、增量构建的既有行为完全向后兼容。
- 资源管线：`postprocess` 步骤总是会执行；不会改写的链接（外链、`<a>`、`<link rel="alternate">`、`<form action>`）保持原值。
- `serve`：`--watch` 默认会清孤儿（v1.4.0 的"删 post 留下旧 HTML"问题被修复）；保留 v1.4.0 行为的用户请显式 `--no-clean-on-watch`。
- 日志：默认 `level=info, format=text, output=stderr`，与 v1.4.0 字节级一致；新增 `logging` 段与 `GOBIN_LOG_*` 均为可选。
- 库 API：新增 `generator.NewAssetFingerprinter` 与 `config.LoggingConfig`；`config.LoadIfPresent` 取代裸 `Load` 在"可能在站点外"的代码路径上的使用；其余签名不变。

---

## 验证

发布前建议执行：

```bash
make test
GOCACHE=/private/tmp/micro-one-api-gocache go test ./...
make lint           # 在 ~/Library/Caches 可写环境
GOCACHE=/private/tmp/micro-one-api-gocache go vet ./...
GOCACHE=/private/tmp/micro-one-api-gocache make release-local
```
