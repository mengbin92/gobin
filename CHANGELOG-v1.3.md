# Gobin v1.3.0 更新日志

## 发布日期 - 2026-06-01

Gobin v1.3.0 是一次向后兼容的功能版本，重点引入并行页面渲染，进一步缩短多核机器上的全量构建时间。

---

## 新增功能

### 并行构建

- 新增 `gobin build --jobs N`，使用多个 worker 并发渲染文章页、列表页和 taxonomy 页面。
- `--jobs 0`（默认）按 CPU 数自动选择并封顶为 4，`--jobs 1` 强制串行，显式 `--jobs N` 不封顶。
- 共享的 `*template.Template` 仅做只读渲染，每个页面写入独立输出路径；并行只改变写盘顺序、不改变内容，产物与串行字节级一致。
- 与 `--incremental` 正交叠加：增量先筛除未变化页面，并行再加速剩余渲染。
- `gobin serve` watcher 重建自动使用自动并发度，无需额外标志。
- 新增 `GenerationOptions.Concurrency` 字段，供库调用方控制并发度。

---

## 改进

- 修复 `assetURL` 模板函数的指纹 hash 缓存在并发渲染下的数据竞争（`go test ./... -race` 通过）。
- 新增并行渲染等价性、统计、错误传播单元测试与并发度封顶基准。

---

## 兼容性

- 本版本保持配置和内容结构向后兼容。
- 既有库入口（`Generate` / `GenerateWithPages` / `GenerateWithPagesResult`）签名不变，默认采用自动并发度，行为对调用方透明。

---

## 性能

页面渲染以写入大量小文件为主、偏 I/O 密集，因此默认并发度封顶为 4，可在多核机器上获得收益而不引入高并发下的文件系统竞争退化。

`BenchmarkBuildFull_Concurrency`（10 核，渲染 + 聚合阶段，文章预解析）：

| 文章数 | 串行（jobs=1） | jobs=4 | 自动（jobs=0，封顶 4） |
| --- | --- | --- | --- |
| 100 | 22.0 ms | 19.2 ms | 19.3 ms |
| 500 | 106 ms | 90.7 ms | 88.1 ms（约 1.2x） |

> 此前未封顶的 NumCPU=10 在该基准上慢于串行（约 124 ms / 500 篇），因此默认封顶为 4。

---

## 验证

发布前执行：

```bash
go test ./... -race
make lint
git diff --check
```
