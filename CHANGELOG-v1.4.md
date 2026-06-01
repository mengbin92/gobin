# Gobin v1.4.0 更新日志

## 发布日期 - 2026-06-01

Gobin v1.4.0 是一次向后兼容的功能版本，引入 Hugo 风格的短代码（shortcodes），让作者在 Markdown 正文中用简短指令生成结构化 HTML，而无需开启全局 `allowUnsafeHTML` 或重复手写 HTML。

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

---

## 改进

- 短代码注册表通过 `parser.RenderOptions`（`json:"-"` 字段）注入解析层，保持 parser 无状态、增量构建 env hash 稳定。
- 无注册表时解析路径与既有行为字节级一致，既有内容不受影响。

---

## 兼容性

- 本版本保持配置和内容结构向后兼容。
- 既有库入口签名中，`renderOptionsFromConfig` 改为返回 `(RenderOptions, error)` 以暴露短代码模板加载错误；命令层调用点已同步更新，对最终用户透明。
- 短代码模板位于 `templates/shortcodes` 与主题 `layouts/shortcodes`，天然被增量构建 env hash 的目录树、`serve` 的结构性变更分类与文件监听覆盖：编辑短代码会触发增量失效与 `serve` 全量重载，无需新增配置或标志。

---

## 验证

发布前执行：

```bash
go test ./... -race
make lint
git diff --check
```

并以 `gobin init` 站点手测：`{{< youtube >}}` / `{{< figure >}}` 正确展开、代码围栏内不展开、未知短代码中断构建、`serve` 编辑短代码触发全量重载。
