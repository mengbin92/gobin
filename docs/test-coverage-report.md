# 测试覆盖报告

## 概述

本报告用于描述 Gobin 当前测试布局、已覆盖的核心行为，以及仍需继续补强的风险区域。

当前基线验证命令：

```bash
go test ./...
```

截至本次更新，以上命令已通过。

---

## 当前测试布局

### 1. CLI 层

测试文件：

- `cmd/gobin/main_test.go`
- `cmd/gobin/commands/cli_test.go`
- `cmd/gobin/commands/serve_test.go`

已覆盖行为：

- 根命令默认执行构建流程
- `init` 脚手架生成
- `build` 成功路径和缺配置失败路径
- `version` 输出
- `serve` 的静态文件服务行为
- 文件监听路径、过滤规则和重建触发条件

### 2. 配置层

测试文件：

- `internal/config/config_test.go`

已覆盖行为：

- 配置文件加载
- 默认值填充与规范化
- Jekyll 风格配置文件回退
- 输出开关、SEO、评论、分析配置
- 错误处理（缺文件、非法 YAML）

### 3. 解析层

测试文件：

- `internal/parser/parser_test.go`

已覆盖行为：

- Front Matter 解析
- Markdown 渲染
- Slug / URL 基础生成
- 摘要、字数、阅读时间
- 草稿与元数据解析
- 错误处理与边界情况

### 4. 生成层

测试文件：

- `internal/generator/generator_test.go`

已覆盖行为：

- 分页
- 模板加载与渲染
- 主题模板覆盖与站点模板回退
- 文章可见性过滤
- permalink 生成
- taxonomy / feed / sitemap / search / alias / assets 等主要输出路径
- comments / analytics / drafts 组合行为
- official theme 与默认模板的 golden 输出校验

---

## 当前结论

### 已经收敛的风险

- CLI 命令不再主要依赖 `os.Exit`，核心路径可直接测试
- 主题模板现在支持正常覆盖，缺失时回退到站点模板
- 配置默认值已统一收口，不再分散在多处兜底
- `init` 脚手架内容与命令流程已拆分，维护成本明显下降

### 仍需继续补强的区域

- `internal/generator` 仍然是复杂度最高的模块，虽然已有大量行为测试，但职责边界仍可继续拆分
- `serve` 目前主要验证静态服务与监听逻辑，尚未覆盖更完整的长生命周期流程
- 历史文档中曾记录过旧的覆盖率数字和测试文件路径；这份报告以后应以“测试布局 + 风险结论”为主，避免再次快速过期

---

## 后续建议

1. 为 `serve` 增加更完整的端到端测试夹具，覆盖首次构建、监听重建和失败恢复路径。
2. 继续拆分 `internal/generator`，把页面渲染、站点产物、内容过滤进一步分层。
3. 如果后续继续追踪覆盖率，建议在 CI 中生成报告并自动回填，避免手工维护静态百分比。
