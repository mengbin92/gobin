# Gobin v1.8.1 发布说明

## 发布日期 - 2026-07-14

Gobin v1.8.1 修复 v1.8.0 Jekyll 模板兼容层的变更跟踪缺口。站点根目录的 `_layouts/` 与 `_includes/` 现在同时进入增量构建环境哈希和 `serve --watch` 监听，修改布局或 include 后会可靠地重新渲染页面。

## 修复内容

- `_layouts/` 和 `_includes/` 纳入 `build_env_hash`。
- 修改、新增或删除 Jekyll 兼容模板时，旧增量 manifest 自动失效。
- `gobin serve --watch` 监听两个目录，并把其中的变化视为结构性变更。
- 布局和 include 各自增加端到端增量构建回归测试，验证输出使用最新模板内容。
- watcher 路径、变化分类和环境哈希增加单元测试。

## 升级方式

```bash
go install github.com/mengbin92/gobin/cmd/gobin@v1.8.1
```

Docker 用户：

```bash
docker pull docker.io/mengbin92/gobin:v1.8.1
```

## 兼容性

- 无配置变更。
- 无模板语法变更。
- 无公开 Go API 变更。
- 不使用 `_layouts/` / `_includes/` 的站点行为与 v1.8.0 一致。
- 已有 `.gobin-build.json` 会因新的环境哈希自动失效并完成一次全量渲染。

## 验证

发布前执行：

```bash
go test ./...
go test -race ./...
go vet ./...
make lint
make release-local
```
