# 资源管线使用指南

> Gobin v1.5.0 起把 HTML 引用改写、资产 hash 验证、文件名指纹默认扩展名列表三件事收口。本指南面向站点作者与 CI 维护者。

## 1. v1.5.0 新增了什么

- **HTML 引用改写**：当 `assets.fingerprint.strategy = filename` 时，渲染后 Gobin 会再扫一遍 `publishDir` 下的 `.html`，把里面的 `<link href>` / `<script src>` / `<img src>` 重写为指纹化后的路径。这意味着你模板里**写死的字符串资源**（如 `href="/css/site.css"`）也会被自动指向 `css/site.abc.css`，不会留下 404。
- **`gobin check --assets`**：发布前的资产 hash 一致性检查。它读 `.gobin-assets.json` manifest，验证每个指纹化文件的磁盘内容 hash 与其文件名中嵌入的 hash 一致。
- **默认指纹扩展名扩充**：默认列表加入 `.mjs`、`.avif`。

## 2. 快速上手

```yaml
# config.yaml
assets:
  fingerprint:
    strategy: filename
    # extensions 可省略；省略时走 DefaultAssetsFingerprintExtensions()
```

构建并验证：

```bash
gobin build --minify
gobin check --assets
# 期望: 1+ fingerprinted assets verified, 0 mismatches
```

## 3. 引用改写覆盖范围

| 场景 | 改写? |
|------|------|
| `<link href="/css/site.css">` | ✓ |
| `<script src="/js/app.js">` | ✓ |
| `<img src="/img/cover.png">` | ✓ |
| `<a href="/some-page/">` | ✗（避免误改站外 / 站内跳转） |
| `<link rel="alternate" href="...">` | ✗（feed/atom 引用，不需要 cache-bust） |
| `<form action="...">` | ✗ |
| `href="https://..."` / `href="//cdn..."` | ✗（跨站） |
| `<link HREF="...">`（大写属性名）| ✓（大小写不敏感） |

未匹配的链接不会被改写，也不会被删；如果磁盘上找不到链接，行为是 404（不归 Gobin 管）。

## 4. `gobin check --assets` 输出

```
  [OK]   verified 14 fingerprinted asset(s) in public
```

或检测到不一致：

```
  [FAIL] css/site.aaaaaaaaaaaa.css: hash mismatch (embedded=aaaaaaaaaaaa, actual=bb1234...)
```

退出码 0 = 全部一致，1 = 有 mismatch 或其它错误。

典型使用：CI 在部署前跑一次，确保构建产物与发布资产一致：

```yaml
- run: gobin build --minify
- run: gobin check --assets
```

## 5. 排错

| 症状 | 可能原因 | 处理 |
|------|----------|------|
| 模板里 `href="/css/site.css"` 没改写 | 该路径未被指纹化（扩展名不在 `Extensions` 配置里） | 把 `.css` 加到 `assets.fingerprint.extensions` |
| `<a href="...">` 改写了 | 应当不会改写 | 提 issue，附 HTML 片段 |
| 跨站链接被改写 | 应当不会改写 | 同上 |
| `check --assets` 报 hash mismatch | 文件被外部修改 / 不同 CI runner 哈希结果不一致（极少见，sha256 稳定）| 重新 `gobin build`；不要手工 `cp` 进 `publishDir` |
| `check --assets` 说"missing manifest" | publishDir 下没有 `.gobin-assets.json` | 先 `gobin build` 一次；或者用的是 `query` 策略（filename 才会写指纹 manifest） |

## 6. 趋势化基准门禁

`scripts/check-benchmark.sh` 在 v1.5.0 同时检查绝对上限与相对回归：

- 绝对上限（不变）：5 个核心 benchmark 的 order-of-magnitude 阈值
- 相对回归（新增）：当前 ns/op > `1.5x` 上次 `HEAD:benchmark-results.txt` → 红；> `1.2x` → 黄

跳过条件：

- 多次 `count=N` 运行的 `(max-min)/mean > 0.30`（基准噪声过大）
- `HEAD` 没有 `benchmark-results.txt`（首次发版 / 切换基线）

CI 默认 `GOBIN_BENCH_FAIL_ON_WARN=1`，warn 也算 fail；本地手跑可设 `GOBIN_BENCH_FAIL_ON_WARN=0` 只看 warn。

调整阈值：

```bash
GOBIN_BENCH_FAIL_RATIO=2.0 GOBIN_BENCH_WARN_RATIO=1.3 make benchmark-check
```
