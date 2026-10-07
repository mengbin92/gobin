# Gobin v1.8.5 发布说明

## 发布日期 - 2026-10-07

Gobin v1.8.5 是一次全量代码审查后的**缺陷修复版本**，在 v1.8.4 之上修复 8 个真实 bug，覆盖 slug 生成、Jekyll 模板解析、开发服务器 LiveReload、日志优先级、配置加载、HTML 压缩和图片管线。无新功能、无配置或公开 API 变更。

## 修复内容

### slug 日期前缀误截断（parser）

- 旧逻辑只判断"文件名第 11 字节是 `-`"就剥离前 11 个字符：`hello-v1.2-release.md`（第 11 位恰好是连字符但不是日期）会被错误截断为 slug `release`。
- 修复后仅当文件名匹配真正的 `YYYY-MM-DD-` 日期前缀时才剥离。

### `_layouts`/`_includes` 探测解析补齐模板函数表（templates）

- v1.8.4 已修复"解析失败的 layout 残留空模板"问题，但其 scratch 模板未携带站点 funcMap，导致**合法**使用 `render`/`safeHTML`/`dateFormat` 等模板函数的 layout 文件会因"函数未定义"被误跳过。
- v1.8.5 的探测解析携带与主模板集相同的 funcMap，解析成功后再经 `AddParseTree` 嫁接，合法 layout 不再被误判。

### LiveReload SSE 连接每 10 秒被掐断（serve）

- 开发服务器 `WriteTimeout: 10s` 同样作用于 Server-Sent Events 长连接，导致 LiveReload 通道周期性断开、浏览器不停重连。
- 修复后 SSE 处理器显式清除写超时，连接保持存活。

### `--log-format` 默认值覆盖配置（logging）

- flag 默认值 `"text"` 使用户未传 flag 时也永远覆盖 `logging.format` 配置和 `GOBIN_LOG_FORMAT`，v1.5.0 声明的 flag > env > config > default 优先级实际失效。
- 修复后 flag 默认为空（未设置），空值仍解析为 text handler；显式传 `--log-format` 的优先级不变。

### `_config.yml` 解析错误被吞（config）

- `LoadIfPresent` 出错时只检查 `config.yaml`/`config.yml` 是否存在；Jekyll 风格站点只有 `_config.yml` 且内容损坏时，错误被静默当成"无配置"。
- 修复后检查全部 4 个候选配置文件，损坏配置的错误正常透出。

### HTML 压缩破坏内联元素间距与未闭合注释（minify）

- 边界空格丢失：`<p>Hello <a>world</a> again</p>` 压缩后渲染成 "Helloworldagain"。修复后按相邻标签的内联/块级性质决定是否保留单个边界空格（内联边界保留、块级边界照旧丢弃）。
- 未闭合的 `<!--` 注释会丢弃文件剩余全部内容。修复后保留剩余内容原样输出。

### 图片管线不支持多 staticDirs（images）

- `resolveSourcePath` 只搜索主 `staticDir`：v1.8.2 引入的额外 `staticDirs` 中的图片全部解析失败、计入 Errors。修复后按与 `collectStaticAssetFiles` 相同的前缀语义搜索所有静态目录（主目录映射到站点根，额外目录保留基名前缀）。
- 外部图片 URL（`http://`、`https://`、`//cdn`）此前被计入 Errors，与文档声明的"跳过"矛盾；修复后直接跳过不计数。

## 测试

每个修复均附带回归测试（共 14 个新测试用例），发布前执行：

```bash
go test ./...
go test -race ./internal/parser/... ./internal/generator/... ./cmd/gobin/commands/...
go vet ./...
```

## 兼容性

- 配置、模板语法、CLI、公开 Go API 均无变更。
- 修复均收紧错误行为或恢复文档声明的行为，不改变任何正确输入的输出。
