# 统一日志系统使用指南

> Gobin v1.4.0 起把诊断日志统一到 `log/slog`，v1.5.0 加入 `config.yaml` 段与 `GOBIN_LOG_*` env var 覆盖。本指南说明 stdout / stderr / 文件分别承担什么、`--verbose` / `--log-format` / `--log-file` 怎么用、CI 怎么把日志和用户输出分离，以及 v1.5.0 的优先级。

## 1. 设计目标

Gobin 把 CLI 的输出分成两类，**必须**清晰分离：

| 通道 | 用途 | 默认目标 |
|------|------|----------|
| 用户输出 | 版本号、计数、`[OK]` / `[FAIL]`、成功提示 | **stdout** |
| 诊断日志 | DEBUG / INFO / WARN / ERROR 级别的结构化日志 | **stderr**（或 `--log-file` / `logging.file`） |

设计原因：

- `gobin build > site.tar.gz` 只打包产物，不混入诊断噪音
- CI 抓日志统一用 `2>`
- 用户脚本可以用 `[ -z "$(gobin check 2>&1 >/dev/null)" ]` 这种简单写法判断成功

## 2. 优先级（v1.5.0）

```
CLI flag > env var > config.yaml > 默认值
```

| 来源 | 覆盖对象 |
|------|----------|
| `--verbose` / `--log-format=...` / `--log-file=...` | 一切 |
| `GOBIN_LOG_LEVEL` / `GOBIN_LOG_FORMAT` / `GOBIN_LOG_FILE` | config 段 |
| `config.yaml` 的 `logging` 段 | 默认值 |
| 缺省 | level=info, format=text, output=stderr |

## 3. `config.yaml` 配置段

```yaml
logging:
  level: info       # debug | info | warn | error
  format: text      # text | json
  file: ""          # 空 → stderr；非空 → 追加写
```

字段含义同 CLI flag。配置段的值会被 CLI flag / env var 覆盖。

### 校验差异

| 来源 | level 非法 | format 非法 | file 不可写 |
|------|-----------|-------------|-------------|
| config.yaml 段 | **启动报错** | **启动报错** | **启动报错** |
| env var（被 config 解析层读到） | **启动报错** | **启动报错** | **启动报错** |
| CLI flag | 容错回退 info | 容错回退 text | 容错回退 stderr 并 WARN |

理由：配置文件是项目级设置，typo 应该被看到；CLI flag 是临时调试，宽容更好。

## 4. 全局标志

三个全局 flag 对所有命令生效（`build` / `serve` / `init` / `new` / `check` / `version`）：

| 标志 | 取值 | 默认 | 说明 |
|------|------|------|------|
| `--verbose` / `-v` | bool | false | 把日志级别提到 DEBUG，并附带源码位置（`source=file.go:123`） |
| `--log-format` | `text` / `json` | `text` | 终端友好文本 / 机器可解析 JSON |
| `--log-file` | 文件路径 | 空（→ stderr）| 把诊断日志追加写文件 |

例：

```bash
# 调试模式：DEBUG + 源码位置 + 文本格式
gobin build --verbose

# CI：JSON 写到文件
gobin build --log-format=json --log-file=build.log

# 终端 JSON 便于 pipe 给 jq
gobin build --log-format=json 2> >(jq -c 'select(.level=="ERROR")')
```

## 5. env var 形式

| env var | 等价 flag |
|---------|-----------|
| `GOBIN_LOG_LEVEL=debug` | `--verbose` |
| `GOBIN_LOG_FORMAT=json` | `--log-format=json` |
| `GOBIN_LOG_FILE=build.log` | `--log-file=build.log` |

典型 docker / CI 部署用法：

```yaml
# docker-compose.yml
services:
  gobin:
    environment:
      GOBIN_LOG_LEVEL: info
      GOBIN_LOG_FORMAT: json
      GOBIN_LOG_FILE: /var/log/gobin.log
    command: ["gobin", "serve", "--port", "8080"]
```

env var 覆盖 `config.yaml`，但被 CLI flag 覆盖。空字符串视为"未设置"。

## 6. 级别

| 级别 | 何时出现 | 例子 |
|------|----------|------|
| DEBUG | 解析进度、模板路径、worker 调度 | `"posts"=10 "pages"=2` |
| INFO | 关键阶段开始/完成 | `"site build completed" elapsed=120ms` |
| WARN | 可恢复异常 | 资源复制路径无法解析，回退默认 |
| ERROR | 不可恢复错误（命令会以非 0 退出）| `failed to load site build input error=...` |

DEBUG 在不开 `--verbose` 时**完全不打**。

## 7. 结构化字段

每条日志都是 `level`、`msg` + 任意键值对。常见字段：

| 字段 | 出处 | 例子 |
|------|------|------|
| `component` | `internal/log.WithComponent` | `generator` / `parser` / `config` |
| `version` / `commit` / `build_date` | `commands` 层启动事件 | `version=v1.5.0` |
| `elapsed` | 关键阶段结束事件 | `elapsed=120.5ms` |
| `error` | 错误日志 | `error="yaml: line 5: ..."` |
| `posts` / `pages` / `output_dir` | generator 入口 | `posts=120` |

`--verbose` 模式下额外带 `source=path/to/file.go:line`，方便定位。

## 8. CI 集成

### 8.1 抓错误并 fail build

```yaml
- name: Build site
  run: |
    gobin build --log-format=json 2> build.log
    if grep -q '"level":"ERROR"' build.log; then
      echo "::error::gobin emitted ERROR level log; see build.log"
      exit 1
    fi
```

### 8.2 分离 stdout / stderr

```bash
# 产物 stdout，诊断 stderr，分开存
gobin build 1>site.stdout 2>site.stderr
```

CI 步骤：

```yaml
- name: Build
  run: gobin build --incremental --clean=false
- name: Upload logs
  if: always()
  uses: actions/upload-artifact@v4
  with:
    name: gobin-logs
    path: |
      site.stdout
      site.stderr
```

### 8.3 写 JSON 到文件供长期分析

```bash
gobin build --log-format=json --log-file=/var/log/gobin.json
```

`--log-file` 是**追加**写，路径不可写会**回退**到 stderr 并打一条 WARN（CLI 来源）或**报错**（config 来源），不让构建崩。

## 9. 内部可观测性（`component` 标签）

Gobin 的 `internal/generator`、`internal/parser`、`internal/config` 在关键路径埋了点。默认级别（INFO）下能看到的：

```
level=INFO msg="content parsed" component=parser posts=10 pages=2
level=INFO msg="site build completed" component=generator pages_rendered=10 pages_skipped=0
```

`--verbose` 模式（DEBUG）下还会看到：

```
level=DEBUG msg="loading site build input" component=generator
level=DEBUG msg="preparing generation plan" component=generator posts=10 incremental=false
level=DEBUG msg="generation plan executed" component=generator pages_rendered=10 pages_skipped=0
```

按 component 过滤：

```bash
gobin build --verbose 2>&1 \
  | grep '"component":"generator"' \
  | jq -c 'select(.level=="DEBUG")'
```

## 10. 排错

| 症状 | 可能原因 | 处理 |
|------|----------|------|
| `gobin build > out.tar` 把诊断日志也带进 tar | 旧版本混用 | 升级到 v1.4.0+；或 `gobin build 2>/dev/null > out.tar` 显式丢 stderr |
| 期望看到 DEBUG 但没输出 | 默认 INFO | 加 `--verbose` / `-v` |
| `--log-file /readonly/x.log` 报权限错 | 文件不可写 | 修路径；CLI 来源下会自动回退 stderr 并打 WARN |
| `logging.level: trace` 启动报 `invalid logging level` | config 来源严格 | 改成合法值（debug/info/warn/error） |
| `logging.file: /nonexistent/x.log` 启动报 not writable | 父目录不存在 | 提前 `mkdir -p` 或改成 stderr |
| 升级后日志字段名变了 | 跨大版本字段可能调整 | 看 `CHANGELOG` 的 "Internal" 段；Gobin 不保证内部结构稳定，但保证 `level` / `msg` / `component` 不变 |
| `gobin check --verbose` 与 `gobin build --verbose` 输出格式不一样 | 命令各自有自己的启动事件 | 是设计如此；只把 `level` / `msg` / `component` 当作稳定 schema |

## 11. 与 `--quiet` / `--silent` 之类的兼容性

Gobin 当前**没有** `--quiet` / `--silent` 标志。用户态"静默构建"的标准做法：

```bash
# 1. 不看用户输出（保留诊断）
gobin build 1>/dev/null

# 2. 静默到文件
gobin build --log-file=build.log 1>/dev/null

# 3. 真的什么都不看（错误也吞）
gobin build 1>/dev/null 2>&1
```
