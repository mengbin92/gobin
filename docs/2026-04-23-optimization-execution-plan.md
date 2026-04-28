# Gobin ROI 优化执行计划

## 1. 目标

本文档用于承接当前项目按投入产出比排序的优化排期，并跟踪 P0 阶段的实际执行进度、验收结果与提交记录。

执行原则：

- 优先处理低投入、高收益的问题
- 每项改动先补失败测试，再补实现
- 每个阶段完成后更新文档状态

## 2. ROI 排期

### P0：立即执行

1. 配置校验收口
   - 状态：已完成
   - 目标：为 `baseURL`、`permalinks`、主题目录、输出目录冲突提供显式校验
   - 预期收益：尽早失败，降低用户排错成本

2. 模板和资源路径规则收口
   - 状态：已完成
   - 目标：减少模板加载和样式路径探测中的隐式行为，建立可测试的路径约定
   - 预期收益：降低主题覆盖和资源引用的漂移风险

3. `internal/generator` 边界再拆一层
   - 状态：已完成
   - 目标：把生成计划组装逻辑进一步显式化，减少主流程耦合
   - 预期收益：为后续增量构建、并行构建和资源管线升级铺路

### P1：尽快跟进

1. `serve` 重构为独立 dev server 层
   - 状态：已完成
2. 搜索索引生成去重
   - 状态：已完成
3. 内容模型拆分为原始输入模型和规范化输出模型
   - 状态：已完成

### P2：中期投入

1. 建立 benchmark 和 CI 性能基线
   - 状态：已完成基础入口
   - 说明：`make benchmark` 运行 Go benchmark 并输出 `benchmark-results.txt`；CI 在 push / PR 中运行并上传结果。后续可在结果稳定后增加趋势对比或阈值门禁。
2. 资源管线升级为增量复制、指纹和按类型处理
   - 状态：部分完成
   - 说明：已完成未变化静态资源跳过复制；指纹、manifest、stale asset 清理仍待实现。
3. 增补 `serve` 生命周期端到端测试
   - 状态：已完成基础覆盖

### P3：延后到内核稳定后

1. 增量构建
2. 并行构建
3. 多语言
4. 短代码
5. 图片优化

## 3. P0 执行拆分

### Task P0.1 配置校验收口

- 范围：
  - 校验 `baseURL`
  - 校验 `permalinks.posts`
  - 校验主题目录存在性
  - 校验输出目录不与输入目录冲突
- 测试策略：
  - 先补 `internal/config/config_test.go`
  - 再把校验接入 `Load`
- 状态：已完成
- 完成项：
  - 新增 `Validate` / `ValidateInDir`
  - `Load` 接入配置校验
  - 新增配置校验测试覆盖

### Task P0.2 模板和资源路径规则收口

- 范围：
  - 抽出显式模板文件清单
  - 收口样式路径解析规则
  - 为资源复制和模板引用建立统一入口
- 测试策略：
  - 先补 `internal/generator/generator_test.go`
  - 再调整模板与资源解析实现
- 状态：已完成
- 完成项：
  - 抽出统一静态资源收集入口
  - 样式缺失时不再返回虚假默认路径
  - 默认模板在无样式时不再输出悬空 `link`
  - 新增资源覆盖和样式探测测试

### Task P0.3 生成计划边界拆层

- 范围：
  - 将生成计划组装拆成显式步骤
  - 降低 `prepareGenerationPlan` 的直接职责数
- 测试策略：
  - 先补计划组装层的行为测试
  - 再做接口和函数边界重构
- 状态：已完成
- 完成项：
  - 拆出 `prepareRenderableContent`
  - 拆出 `assembleGenerationPlan`
  - 为内容准备和计划装配新增独立测试

## 4. 进度记录

### 2026-04-23

- 已创建执行文档
- 已完成 Task P0.1：配置校验收口
- 已完成 Task P0.2：模板和资源路径规则收口
- 已完成 Task P0.3：生成计划边界拆层
- P0 阶段已完成
- 已创建 P1 实施计划文档
- 已完成 P1.1：`serve` 重构为独立 dev server 组件
- 已完成 P1.2：搜索索引生成去重
- 已完成 P1.3：内容模型拆分为 front matter 输入模型和规范化运行时模型
- P1 阶段已完成

### 2026-04-27

- 已完成 P2 部分优化：Markdown raw HTML 渲染安全开关
- 已完成 P2 部分优化：`serve` 生命周期与重建失败恢复测试补强
- 已完成 P2 部分优化：资源管线 v1，未变化静态资源跳过复制
- 已完成 P2 部分优化：benchmark 统一入口和 CI 基线产物上传
- `markup.allowUnsafeHTML` 默认保持兼容，显式设为 `false` 时禁用原始 HTML 输出
- 站点构建和 `serve` 的解析入口现在会使用配置派生的 Markdown 渲染选项
- 静态资源复制新增可测试 copy plan，为后续指纹和完整增量构建打底
- `make benchmark` 现在会运行 Go benchmark，输出 `benchmark-results.txt`，CI 会上传该文件作为性能基线产物

## 5. 验收记录

- 2026-04-23
- 验证命令：`go test ./...`
- 验证结果：通过
- 验收结论：
  - 配置加载现在会显式拦截非法 `baseURL`、非法 `permalinks.posts`、缺失主题目录和输出目录冲突
  - 静态资源复制改为统一资源清单驱动
  - 默认模板在无样式资源时不会再输出悬空样式引用
  - 生成计划入口已拆分为“内容准备”和“计划装配”两个显式步骤

- 2026-04-23
- 验证命令：`go test ./...`
- 验证结果：通过
- 验收结论：
  - `serve` 现在拆分为 builder、watcher、server 三个内部组件，命令入口只负责组装流程
  - 搜索索引 full/min 两条输出链路改为共用搜索文档构建器
  - parser 层新增 front matter 输入模型和归一化函数，`ParsePost` / `ParsePage` 对外行为保持不变

- 2026-04-27
- 验证命令：`go test ./internal/config ./internal/parser ./cmd/gobin/commands`
- 验证结果：通过
- 验收结论：
  - 配置层新增 `markup.allowUnsafeHTML` 解析
  - parser 层新增 option-aware Markdown 解析入口，并保留默认 raw HTML 兼容行为
  - command 层站点输入加载会把配置转换为 parser 渲染选项
  - `serve` watcher 覆盖重建失败后继续接受后续重建事件的生命周期行为

- 2026-04-27
- 验证命令：`go test ./internal/generator -run 'TestPlanStaticAssetCopies|TestExecuteStaticAssetCopyPlan|TestCopyStaticAssets'`
- 验证结果：通过
- 验收结论：
  - 静态资源复制现在先生成 copy/skip plan
  - 目标文件已是最新时会跳过复制，避免非 clean rebuild 重写未变化资源
  - 资源变化检测覆盖目标缺失、大小变化、权限变化和源文件更新时间更新
  - `--clean=false` 仍不删除 stale 输出文件，旧文件清理由 clean build 负责

- 2026-04-27
- 验证命令：`make benchmark BENCH_TIME=1x`
- 验证结果：通过
- 验收结论：
  - 本地和 CI 共用 `make benchmark` 入口
  - benchmark 结果写入 `benchmark-results.txt`
  - CI 会上传 benchmark 结果作为性能基线产物

## 6. 提交记录

- `7b9f468` `Complete P0 optimization cleanup`
- `a241ed6` `Update optimization execution record`
- 待补充 P1 统一提交哈希
- 待补充 P2 Markdown safety / serve lifecycle 提交哈希
- 待补充 P2 asset pipeline incremental copy 提交哈希
