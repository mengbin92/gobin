# Gobin v1.4.0 发布说明

## 发布日期 - 2026-06-02

Gobin v1.4.0 是一次向后兼容的功能版本。本次发布在 v1.2.0 的基础上累计合入了 v1.3 与 v1.4 两轮开发成果：并行页面渲染、`serve` 亚秒级增量重建、Hugo 风格短代码，以及基于标准库 `log/slog` 的统一日志系统。

> 说明：v1.3.0 未单独发布 tag，其内容（并行构建、`serve` 增量重建）已并入本次 v1.4.0 发布。从 v1.2.0 升级可一并获得。

---

## 亮点

- **统一日志系统（`log/slog`）**：新增全局 `--verbose`、`--log-format text|json`、`--log-file` 标志。用户输出走 stdout，结构化诊断日志走 stderr 或文件，CI 管道可轻松分离；`generator`/`parser`/`config` 关键路径以 `component=` 标签可观测。
- **Hugo 风格短代码（Shortcodes）**：在 Markdown 正文用 `{{< name args >}}` / `{{% name args %}}` 生成结构化 HTML，内置 `figure`、`youtube`、`gist`、`highlight`，支持站点/主题自定义覆盖，无需开启全局 `allowUnsafeHTML`。
- **并行构建**：`gobin build --jobs N` 多 worker 并发渲染，`--jobs 0`（默认）按 CPU 自动选择并封顶为 4；产物与串行字节级一致。
- **`serve` 亚秒级增量重建**：文件保存时只重新解析变化的内容文件，其余从会话缓存复用，消除每次保存对全站 Markdown 的全量重解析。

---

## 升级方式

```bash
go install github.com/mengbin92/gobin/cmd/gobin@v1.4.0
```

或从 GitHub Releases 下载对应平台的压缩包，并使用 `SHA256SUMS` 校验。

Docker 用户可以使用：

```bash
docker pull docker.io/mengbin92/gobin:v1.4.0
```

---

## 统一日志系统

默认行为与既有版本一致——stdout 只显示面向用户的输出，诊断日志（INFO 级别）写到 stderr：

```bash
# 默认：用户输出在 stdout，诊断日志在 stderr
gobin build

# 调试：DEBUG 级别 + 源码位置
gobin build --verbose

# CI 管道：JSON 日志写文件，stdout 保持纯净
gobin build --log-format json 2>build.log

# 排查：详细日志持久化到文件（追加写）
gobin build --verbose --log-file gobin-debug.log
```

| 输出目标 | 内容 |
| --- | --- |
| stdout | 面向用户的信息（版本号、统计、`[OK]`/`[FAIL]`、成功提示） |
| stderr / 文件（slog） | 结构化诊断日志，带 `level`、`msg`、`component` 等键 |

日志级别：DEBUG（`--verbose` 才开启）/ INFO（默认）/ WARN / ERROR。

---

## 短代码

```markdown
{{< youtube id="dQw4w9WgXcQ" >}}

{{< figure src="/img/a.png" caption="示意图" >}}

{{< highlight go >}}
fmt.Println("hello")
{{< /highlight >}}
```

- `{{< … >}}` 输出原始 HTML（即使 `markup.allowUnsafeHTML: false` 也生效）；`{{% … %}}` 输出再经 Markdown 渲染。
- 自定义：站点 `templates/shortcodes/<name>.html` 或主题 `<theme>/layouts/shortcodes/<name>.html`，优先级 站点 > 主题 > 内置。
- 代码围栏与行内代码内不展开；引用未注册短代码会中断构建并指出文件与名称。

---

## 并行构建与 serve 增量重建

```bash
# 并行渲染（自动并发度，封顶 4）
gobin build

# 显式指定 worker 数
gobin build --jobs 8

# 强制串行
gobin build --jobs 1
```

`gobin serve` 的 watcher 重建默认走增量路径，`--verbose` 下打印 `Partial rebuild: N changed, M reused` / `Full reload: N source(s) parsed`，便于观察重建路径。

---

## Docker 镜像

Git tag 发布时会同时构建并推送 Docker Hub 镜像：

- `docker.io/mengbin92/gobin:v1.4.0`
- `docker.io/mengbin92/gobin:latest`

镜像支持：

- `linux/amd64`
- `linux/arm64`

运行示例：

```bash
docker run --rm -p 8080:8080 \
  -e GOBIN_AUTO_INIT=true \
  -v "$PWD:/site" \
  docker.io/mengbin92/gobin:v1.4.0
```

---

## 兼容性说明

- 本版本保持配置、内容结构和模板接口向后兼容。
- 日志系统默认级别 INFO、格式 text、输出 stderr；不传任何新标志时 stdout 用户可见输出与既有行为完全一致。`--verbose` / `--log-format` / `--log-file` 均为可选。
- `serve` 原有的局部 `--verbose` 已统一为全局标志，语义一致。
- 短代码注册表通过 `parser.RenderOptions` 注入，无注册表时解析路径与既有行为字节级一致；`renderOptionsFromConfig` 改为返回 `(RenderOptions, error)`，对最终用户透明。
- 并行渲染与 `serve` 增量重建为内部优化，既有库入口签名不变，产物与串行/全量构建字节级一致。

---

## 验证

发布前建议执行：

```bash
go test ./... -race
make lint
git diff --check
```
