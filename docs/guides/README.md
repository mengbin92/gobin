# 用户指南

本目录面向 Gobin **使用者**（站点作者、CI 维护者、主题作者），讲解 v1.4.0 引入或收口的几项关键能力。

| 指南 | 适用对象 | 涉及版本 |
|------|----------|----------|
| [Shortcodes 使用指南](./shortcodes.md) | 站点作者 / 主题作者 | v1.4.0+ |
| [增量构建使用指南](./incremental-build.md) | 站点作者 / CI 维护者 | v1.4.0+ |
| [并行构建使用指南](./parallel-build.md) | 大型站点 / 性能调优 | v1.4.0+ |
| [统一日志系统使用指南](./logging.md) | 所有人 / CI 维护者 | v1.4.0+ |

> 设计文档（specs）与实施计划（plans）放在 `docs/superpowers/` 和 `docs/` 根目录。
> 用户指南是**面向使用**的，说明"什么时候用、产物怎么变、怎么排错"；设计文档说明"为什么这样设计、边界在哪"。

## 关联阅读

- 增量构建的设计稿：`docs/2026-05-21-incremental-build-design.md`
- 并行构建的实施记录：`docs/2026-04-23-optimization-execution-plan.md` §5（2026-06-01 条目）
- 短代码的实施记录：`docs/2026-04-23-optimization-execution-plan.md` §5 / `CHANGELOG-v1.4.md`
- 统一日志的规格与实施计划：`docs/superpowers/specs/2026-06-02-logging-system-design.md` / `docs/superpowers/plans/2026-06-02-logging-system.md`
