# Gobin v1.8.4 发布说明

## 发布日期 - 2026-08-26

Gobin v1.8.4 修复 **Jekyll `_layouts` 兼容层在文件解析失败时残留空模板，导致整站构建失败**的问题。

## 问题

v1.8.0 引入的 `_layouts`/`_includes` 兼容层在解析失败（如文件仍含 Liquid 语法 `{% if %}`、`{{ site.x }}`）时，本意图是"跳过该文件并降级到下一个候选模板（如 `singlePage`）"。但实现上先用 `tmpl.New(name)` 在主模板集上注册了名字再 Parse，解析失败后该名字仍留在模板集中且内容为空。渲染时 `Lookup("post")` 命中这个空模板，直接报 `html/template: "post" is an incomplete template`，构建中断。

从 v1.2.0 之前版本升级的 Jekyll 站点（含未迁移的 `_layouts/post.html`）升级到 v1.8.x 会因此无法构建。

## 修复

- `registerLayoutsAndIncludes` 改为先在临时模板中解析，成功后才通过 `AddParseTree` 把文件内容（含 `{{ define }}` 块）注册进主模板集；解析失败则真正跳过，不影响后续模板回退。
- 新增回归测试 `TestLoadTemplatesSkipsUnparseableLayoutWithoutRegisteringName`。

## 兼容性

- 无配置项、模板语法或公开 Go API 变更。
- 解析成功的 `_layouts`/`_includes` 文件行为不变。

## 验证

```bash
go test ./...
go vet ./...
```
