# Gobin v1.8.3 发布说明

## 发布日期 - 2026-08-26

Gobin v1.8.3 修复 **Markdown 表格无法渲染**的问题。

## 问题

此前 goldmark 以纯 CommonMark 模式初始化（未启用任何扩展），而表格属于 GFM 扩展语法，导致文章中的 pipe table 被渲染为普通段落，部署后页面上表格显示为原始 `| --- |` 文本。

## 修复

- `internal/parser` 渲染时启用 `extension.GFM`，一次性补齐常用 GFM 语法：
  - **表格（Table）**
  - 删除线（Strikethrough）
  - 自动链接（Linkify）
  - 任务列表（TaskList）
- 新增回归测试 `TestPostRendering_GFMTable`，覆盖中文表头的表格渲染。

## 兼容性

- 无配置项、模板语法或公开 Go API 变更。
- 对已有内容的影响仅限新增语法生效：`|` 表格、~~删除线~~、裸 URL 自动链接、`- [ ]` 任务列表现在会被解析。

## 验证

```bash
go test ./...
go vet ./...
```
