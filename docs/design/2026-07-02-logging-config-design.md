# 配置文件中的 logging 段 — 实现规格 (Spec)

> 日期：2026-07-02
> 状态：草稿，待 review
> 范围：v1.5.0
> 承接：`docs/design/2026-06-02-logging-system-design.md` "明确排除（Phase 4，不在本期）"

## 1. 目标

v1.4.0 logging 系统的 spec 把 `config.yaml` 的 logging 段**显式排除**到 Phase 4。v1.5.0 把它补上：

- 用户在 `config.yaml` 里写 `logging: { level, format, file }` 就能控制诊断日志输出
- CLI flag 仍能覆盖 config（CI 临时改不改 commit）
- env var 也可覆盖 config（docker 部署场景）

## 2. 设计决策

| 决策 | 选择 | 理由 |
|------|------|------|
| config 段结构 | `logging.level` / `logging.format` / `logging.file`，与 CLI flag 名字一致 | 心智模型一致 |
| 优先级（高 → 低） | CLI flag > env var > config.yaml > 默认值 | flag 是临时覆盖，env 是部署覆盖，config 是项目配置 |
| env var 命名 | `GOBIN_LOG_LEVEL` / `GOBIN_LOG_FORMAT` / `GOBIN_LOG_FILE` | 与 docker-compose / k8s 配置习惯一致；与已有 `GOBIN_AUTO_INIT` 命名风格一致 |
| 校验 | level ∈ {debug, info, warn, error}；format ∈ {text, json}；file 可写 / 可创建 | 复用 `internal/log` 已有 `NewFromCLI` 的解析路径，不新增校验函数 |
| 行为 | config + flag 同时设时，flag 胜出 | 不破坏 v1.4.0 CLI 体验 |
| 默认值 | 与 v1.4.0 一样（level=info, format=text, output=stderr） | 零迁移成本 |

## 3. 目录结构

```
internal/config/
  config.go          # 新增 LoggingConfig 段、Logging 字段
  config_test.go     # 新增 logging 段解码测试
internal/log/
  log.go             # 新增 NewFromConfig(cfg.Logging) NewFromEnv()
  log_test.go
cmd/gobin/commands/
  logging.go         # 把 InitLogging 改为优先级解析：flag > env > config > default
  logging_test.go
example-site/
  config.yaml        # 在样例中演示 logging 段
docs/guides/
  logging.md         # 追加 "config.yaml 配置 logging" 段
```

## 4. 行为契约

### 4.1 config.yaml 形式

```yaml
logging:
  level: info       # debug | info | warn | error
  format: text      # text | json
  file: ""          # 空 → stderr；非空 → 追加写
```

### 4.2 优先级

| 来源 | 覆盖对象 |
|------|----------|
| `--verbose` / `--log-format=...` / `--log-file=...` | 一切 |
| `GOBIN_LOG_LEVEL=debug` 等 | config 段 |
| `config.yaml` `logging: { ... }` | 默认值 |
| 缺省 | level=info, format=text, output=stderr |

实现：

```go
// 伪代码
func resolveLogging(cfg *config.Config, flags *FlagValues) *slog.Logger {
    level, format, file := cfg.Logging.Level, cfg.Logging.Format, cfg.Logging.File
    if v := os.Getenv("GOBIN_LOG_LEVEL"); v != "" { level = v }
    if v := os.Getenv("GOBIN_LOG_FORMAT"); v != "" { format = v }
    if v := os.Getenv("GOBIN_LOG_FILE"); v != "" { file = v }
    if flags.Verbose { level = "debug" }
    if flags.Format != "" { format = flags.Format }
    if flags.File != "" { file = flags.File }
    return log.New(log.Options{Level: level, Format: format, Output: file})
}
```

### 4.3 校验

- `level` 不在白名单 → 启动报错（v1.4.0 行为是不识别值回退 info；v1.5.0 在 config 来源下**报错**，flag 来源下保持原宽容行为）
- `file` 路径父目录不可创建 → 启动报错（v1.4.0 行为是回退 stderr 并 WARN；v1.5.0 在 config 来源下**报错**）

理由：配置文件是项目级设置，作者应该知道 typo；flag 是临时调试，宽容更好。

## 5. 不在范围

- 动态 reload（修改 config.yaml 后不重启生效）—— v1.5.0 不做
- 按 `component` 标签的级别过滤 —— v1.5.0 不做
- 滚动日志文件 —— v1.4.0 显式不做，v1.5.0 仍不做
- HTTP 访问日志 —— v1.4.0 显式不做，v1.5.0 仍不做
- 各阶段耗时性能报告 —— v1.4.0 显式不做，v1.5.0 仍不做

## 6. 风险

- v1.4.0 文档 / 教程可能没说"flag 覆盖 config"；新加的优先级要在 docs/guides/logging.md 顶部讲清楚
- env var 命名 `GOBIN_*` 是新引入的；与已有 `GOBIN_AUTO_INIT` 风格一致，但 `GOBIN_LOG_LEVEL` 这种"长名字 + 大写"对 Windows 用户不友好（Windows 区分大小写）；可接受

## 7. 验证

```bash
# 解码
go test ./internal/config -run 'TestDecodeLoggingConfig'

# 优先级
go test ./cmd/gobin/commands -run 'TestResolveLogging_Priority'
# - flag > env > config > default
# - flag 不设、env 设、config 设 → env 胜
# - 全部不设 → default
# - 非法 level 在 config 来源下报错

# 端到端
GOBIN_LOG_LEVEL=debug gobin build 2> build.stderr
grep '"level":"DEBUG"' build.stderr | head -1
# 期望: 至少一条 DEBUG 日志
```
