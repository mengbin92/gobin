# Gobin v1.4.0 更新日志

## 发布日期 - 2026-06-02

Gobin v1.4.0 是一次向后兼容的功能版本，引入两项能力：Hugo 风格的短代码（shortcodes），让作者在 Markdown 正文中用简短指令生成结构化 HTML，而无需开启全局 `allowUnsafeHTML` 或重复手写 HTML；以及基于标准库 `log/slog` 的统一日志系统，将用户输出与诊断日志严格分离，并为内部各层提供可观测性。

---

## 新增功能

### 短代码（Shortcodes）

- 支持两种分隔符形式：
  - `{{< name args >}}` —— HTML 形式，模板输出作为原始 HTML 注入，即使 `markup.allowUnsafeHTML: false` 也生效（通过渲染期 sentinel 占位，绕过 Markdown 转义）。
  - `{{% name args %}}` —— Markdown 形式，模板输出再经 Markdown 渲染。
- 支持配对形式 `{{< name >}}body{{< /name >}}`，正文通过 `.Inner` 提供，并先行递归展开嵌套短代码。
- 参数支持位置参数与引号命名参数（`key="val"`，含 `\"` 转义）混用；模板上下文提供 `.Get`（int 取位置、string 取命名）、`.Inner`、`.Name` 及 `safeHTML` / `absURL` / `urlize` / `default` 辅助函数。
- 内置 4 个短代码：`figure`、`youtube`、`gist`、`highlight`（以嵌入模板定义，可被用户覆盖）。
- 自定义与覆盖：站点 `templates/shortcodes/<name>.html`、主题 `<theme>/layouts/shortcodes/<name>.html`，优先级为站点 > 主题 > 内置，与模板覆盖规则一致。
- 代码围栏与行内代码中的短代码语法不会展开。
- 引用未注册的短代码会中断构建并指出文件与名称（fail-fast）。

### 统一日志系统（基于 `log/slog`）

- 新增 `internal/log` 包：零外部依赖，封装标准库 `log/slog`，提供 logger 工厂、全局默认 logger 与 context 传播。
- 新增三个全局标志（对所有命令生效）：
  - `--verbose` / `-v` —— 将日志级别提升至 DEBUG，并附带源码位置（`source=`）。
  - `--log-format text|json` —— 终端友好的文本格式（默认）或机器可解析的 JSON 格式。
  - `--log-file <path>` —— 将日志写入文件（追加模式）；路径不可写时回退到 stderr 并给出提示。
- **用户输出与诊断日志严格分离**：面向用户的信息（版本号、统计、`[OK]`/`[FAIL]`、成功提示）仍写 stdout；诊断日志走 slog（默认 stderr，或文件）。CI 管道可用 `2>` 轻松分离。
- **结构化与分级**：每条日志带 `level`、`msg` 及结构化键值；DEBUG（调试细节）/ INFO（关键事件）/ WARN（可恢复异常）/ ERROR（不可恢复错误）四级。
- **内部层可观测**：`generator`、`parser`、`config` 关键路径以 `component=` 标签埋点（DEBUG 追踪 + INFO 完成事件），内部包只返回错误、由命令层统一记录，避免重复日志。
- 统一 `serve` 原有的局部 `--verbose` 与全局标志，消除冲突。

---

## 改进

- 短代码注册表通过 `parser.RenderOptions`（`json:"-"` 字段）注入解析层，保持 parser 无状态、增量构建 env hash 稳定。
- 无注册表时解析路径与既有行为字节级一致，既有内容不受影响。
- 替换命令层散落的 `fmt.Fprintf(stderr, ...)` 诊断输出与唯一一处 `log.Printf`，全部改走结构化 slog。

---

## 兼容性

- 本版本保持配置和内容结构向后兼容。
- 既有库入口签名中，`renderOptionsFromConfig` 改为返回 `(RenderOptions, error)` 以暴露短代码模板加载错误；命令层调用点已同步更新，对最终用户透明。
- 短代码模板位于 `templates/shortcodes` 与主题 `layouts/shortcodes`，天然被增量构建 env hash 的目录树、`serve` 的结构性变更分类与文件监听覆盖：编辑短代码会触发增量失效与 `serve` 全量重载，无需新增配置或标志。
- 日志系统默认级别为 INFO、格式为 text、输出到 stderr，不配置任何标志时 stdout 的用户可见输出与既有行为完全一致；新增的 `--verbose` / `--log-format` / `--log-file` 均为可选标志。

---

## 验证

发布前执行：

```bash
go test ./... -race
make lint
git diff --check
```

并以 `gobin init` 站点手测：`{{< youtube >}}` / `{{< figure >}}` 正确展开、代码围栏内不展开、未知短代码中断构建、`serve` 编辑短代码触发全量重载；以及 `gobin build`（stdout 无 `level=` 行）、`gobin build --verbose`（stderr 出现 DEBUG/INFO 且带 `component=`）、`--log-format json`（合法 JSON 行）、`--log-file`（追加写、坏路径回退 stderr）。
