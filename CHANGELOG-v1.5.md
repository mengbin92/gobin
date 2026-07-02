# Gobin v1.5.0 更新日志

## 发布日期 - 2026-07-02

Gobin v1.5.0 是一次面向构建产物与运行时一致性的功能版本。资源管线补齐了"指纹化后 HTML 引用改写"和"产物 hash 验证"两个收口动作；`serve` 的 watcher 重建默认行为对齐到 `gobin build --clean=true`，删文章/资源不再留下孤儿；统一日志系统在 v1.4.0 的全局标志之上加入 `config.yaml` 的 `logging` 段与 `GOBIN_LOG_*` 环境变量覆盖。本次发布保持配置、模板、CLI 入口、并行构建、增量构建的既有行为完全向后兼容。

---

## 新增功能

### 资源管线收口：HTML 引用改写 + 资产 hash 验证

- 渲染后新增 `postprocess` 产物：构建管线走完 minify 之后再扫一遍 `publishDir` 下的 `.html`，把 `<link href>` / `<script src>` / `<img src>` 的值重写为指纹化后的路径。改写映射由 `collectAssetRewriteEntries` 用与拷贝阶段同一个 fingerprinter 构造，模板里写死的 `href="/css/site.css"` 现在自动指向 `css/site.<hash>.css`。
- 重写范围显式界定：外链（`https://`、`//`、`/\`）、`<a href>`、`<link rel="alternate">`、`<form action>` 一律跳过；属性名大小写不敏感。
- 资产重写统计暴露在 `GenerationResult.Postprocess`，便于上层命令报告。
- 新增 `gobin check --assets` 子模式：读 `.gobin-assets.json` manifest，对每个指纹化文件重新算 sha256，验证文件名里嵌入的 hash 与磁盘实际 hash 一致。不一致时退出码 1，适合作为 CI 在 `gobin build` 之后的门禁。
- 默认指纹扩展名 `DefaultAssetsFingerprintExtensions` 扩充：新增 `.mjs`、`.avif`。
- 导出 `generator.NewAssetFingerprinter` 包装函数，让 `commands` 包可以单独构造一个用于 `check --assets` 验证的 fingerprinter。

### `serve` watcher 清理行为对齐 `build --clean=true`

- v1.4.0 的 watcher 重建路径曾硬编码 `runtime.cleanOutput = false`（注释：避免每次保存都清空 publishDir 抹掉刚启动的 manifest），造成删文章/资源后旧 HTML 仍可被站外链接 200 命中。
- v1.5.0 移除该硬编码：watch 路径直接跟随 `serveClean`（默认 `true`），与一次性 `gobin build` 的清理行为对齐。`serveBuilder.rebuildResult` 早已在 `CleanOutput=true` 时丢弃 `Incremental` 字段，二者在 watch 路径上互斥，与 `build` 路径一致。
- 担心"manifest 在每次保存时被清空"是不必要的：增量 manifest 在重建时按需读取而不是每次保存都写；clean 重建结束时自然生成一份新 manifest。
- 新增 `--no-clean-on-watch` flag：恢复 v1.4.0 行为，给另一进程维护 `publishDir` 或跑在慢盘上的用户留逃生口。
- 完整测试覆盖：v1.5.0 默认行为（`WatcherCleansByDefault`）、`--no-clean-on-watch` 恢复 v1.4.0（`NoCleanOnWatchRestoresV140`）、`--clean=false` 的正交路径（`ServeCleanFalseKeepsStaleOutput`）、`WatcherClean` 选项正确下传到 generator、watcher runtime 走增量路径。

### 统一日志系统：`config.yaml` + 环境变量覆盖

- v1.4.0 的统一日志系统规范里把 `config.yaml` 接线明确划为 Phase 4，v1.5.0 把它做完。
- `internal/config` 新增 `LoggingConfig{Level, Format, File}` 与 `Config.Logging` 字段；新增 `LoadIfPresent` 帮助函数，给"可能在站点内也可能在站点外"的代码路径（如 `PersistentPreRun` 处理 `gobin version` 无 config 场景）使用。
- `internal/log` 新增 `NewFromValues`：用叶子结构 `ConfigValues`（刻意不依赖 `config` 包，避免导入循环）构建 logger；并新增 `NewFromEnv` 读取 `GOBIN_LOG_LEVEL` / `GOBIN_LOG_FORMAT` / `GOBIN_LOG_FILE` 作为覆盖。
- 严格校验：未知 `level` / 未知 `format` / 不可写 `file` 在启动期返回错误——把配错项提前到 build 命令前 fail，而不是悄悄回退。
- `cmd/gobin/commands/logging.go` 重写 `InitLogging` 接受 `*config.Config`，按 `flag > env > config > default` 优先级应用；保留 `InitLoggingFromFlags` 给不需要 config 的测试入口。
- `cmd/gobin/main.go` 的 `PersistentPreRun` 改用 `LoadIfPresent + InitLogging`，`gobin version`（无 config）继续工作，`gobin build`（有 config）自动拾取 `logging` 段。
- 完整的 `ConfigValues` 叶子类型也方便未来命令各自接自己的优先级链，无需重写 overlay 逻辑。

---

## 改进

- `BuildArtifactSpecs` 在 `assets` postprocess 启用后由 7 个变 8 个，测试同步更新。
- `gobin check` 子模式把 `--assets` 作为与既有 `--internal-links` 等并列的检查子模式注册，行为可用 `gobin check --help` 看到。
- `serve` 的 `ServeCmd` 长帮助文本现在显式说明 watcher 重建会清理孤儿，并提示 `--no-clean-on-watch` 逃生口。
- `Makefile` 自身的 `release-local` 在 v1.5.0 仍为 6 平台（darwin amd64/arm64、freebsd/amd64、linux/amd64/arm64、windows/amd64）+ `SHA256SUMS`；模块缓存出现 stat 写入失败时会以 warning 形式存在，构建产物不受影响。
- `docs/guides/` 本轮新增 2 篇：`asset-pipeline.md`（v1.5.0 资源管线收口）、`serve.md`（watch 行为变更与 escape hatch）；`logging.md` 更新到 v1.5.0 的优先级与 `config.yaml` 段。

---

## 兼容性

- 配置文件：本版本新增可选顶层 `logging` 段；不写则与 v1.4.0 行为完全一致（INFO / text / stderr）。
- 资源管线：`postprocess` 步骤总是会执行；不会改写的链接（外链、`<a>`、`<link rel="alternate">`、`<form action>`）保持原值，不会引入误改写或删除。
- `serve` 行为：v1.4.0 的 `serve --watch` 在删文章/资源后会留下旧 HTML；v1.5.0 不再有此行为，这是"非回归"修复。需要在 watch 路径上保留 v1.4.0 行为的用户可加 `--no-clean-on-watch`。
- CLI 标志：仅新增 `--no-clean-on-watch`、`gobin check --assets`；既有标志语义不变。
- 库 API：暴露 `generator.NewAssetFingerprinter` 为新导出符号；既有的 `Build` / `NewBuilder` / `IncrementalBuild` 签名不变。

---

## 验证

发布前执行：

```bash
make test
GOCACHE=/private/tmp/micro-one-api-gocache go test ./...
make lint           # 在 ~/Library/Caches 可写环境
GOCACHE=/private/tmp/micro-one-api-gocache go vet ./...
GOCACHE=/private/tmp/micro-one-api-gocache make release-local
```

并手测：

- `gobin build --minify && gobin check --assets` → `[OK] verified N fingerprinted asset(s) in public`，退出码 0。
- 把 `publishDir` 下某个 `.css` 内容改掉再 `gobin check --assets` → `[FAIL] <file>: hash mismatch ...`，退出码 1。
- `gobin serve --watch`，删一个 post → 对应 `publishDir/<date>/<slug>/index.html` 被自动清掉（v1.4.0 不会）。
- `gobin serve --watch --no-clean-on-watch`，删同一个 post → 旧 HTML 留下，验证逃生口。
- `config.yaml` 写 `logging: { level: debug, format: json, file: gobin.log }` + `gobin build` → 三个字段都从配置段读入；`GOBIN_LOG_LEVEL=warn gobin build` 覆盖为 warn；`gobin build --verbose` 覆盖为 debug。
- `config.yaml` 写 `logging: { level: nope }` → 启动报错而不是悄悄回退 info。
