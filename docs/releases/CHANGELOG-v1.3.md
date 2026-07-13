# Gobin v1.3.0 更新日志

## 发布日期 - 2026-06-01

Gobin v1.3.0 是一次向后兼容的功能版本，重点引入并行页面渲染与 `serve` 亚秒级增量重建，进一步缩短多核机器上的全量构建时间和开发态保存后的重建耗时。

---

## 新增功能

### 并行构建

- 新增 `gobin build --jobs N`，使用多个 worker 并发渲染文章页、列表页和 taxonomy 页面。
- `--jobs 0`（默认）按 CPU 数自动选择并封顶为 4，`--jobs 1` 强制串行，显式 `--jobs N` 不封顶。
- 共享的 `*template.Template` 仅做只读渲染，每个页面写入独立输出路径；并行只改变写盘顺序、不改变内容，产物与串行字节级一致。
- 与 `--incremental` 正交叠加：增量先筛除未变化页面，并行再加速剩余渲染。
- `gobin serve` watcher 重建自动使用自动并发度，无需额外标志。
- 新增 `GenerationOptions.Concurrency` 字段，供库调用方控制并发度。

### serve 亚秒级增量重建

- `gobin serve` 的文件监听重建改为 partial rebuild：只重新解析本次变化的内容文件，其余从会话级解析缓存复用，消除每次保存对全站 markdown 的全量重新解析与渲染。
- 变化路径按内容页 / 独立页 / 静态资源 / 结构性自动归类；config、`templates/`、主题目录或内容目录下的非 markdown 文件等结构性变更会回退到全量重新加载，保证正确性。
- 缓存按文件路径字典序输出结构体浅拷贝，产物与全量构建字节级一致；删除或编辑器原子保存（rename-over）以磁盘最终状态为准。
- `--verbose` 下打印 `Partial rebuild: N changed, M reused` / `Full reload: N source(s) parsed`，便于观察重建路径。
- 默认开启，无需额外标志或配置。

---

## 改进

- 修复 `assetURL` 模板函数的指纹 hash 缓存在并发渲染下的数据竞争（`go test ./... -race` 通过）。
- 新增并行渲染等价性、统计、错误传播单元测试与并发度封顶基准。
- 新增 `serve` 增量重建的变更归类、缓存复用、回退路径单元测试，以及 partial rebuild 与全量构建字节一致的集成测试。

---

## 兼容性

- 本版本保持配置和内容结构向后兼容。
- 既有库入口（`Generate` / `GenerateWithPages` / `GenerateWithPagesResult`）签名不变，默认采用自动并发度，行为对调用方透明。
- `serve` 增量重建为内部优化，命令、标志与输出产物均不变，对用户透明。

---

## 性能

页面渲染以写入大量小文件为主、偏 I/O 密集，因此默认并发度封顶为 4，可在多核机器上获得收益而不引入高并发下的文件系统竞争退化。

`BenchmarkBuildFull_Concurrency`（10 核，渲染 + 聚合阶段，文章预解析）：

| 文章数 | 串行（jobs=1） | jobs=4 | 自动（jobs=0，封顶 4） |
| --- | --- | --- | --- |
| 100 | 22.0 ms | 19.2 ms | 19.3 ms |
| 500 | 106 ms | 90.7 ms | 88.1 ms（约 1.2x） |

> 此前未封顶的 NumCPU=10 在该基准上慢于串行（约 124 ms / 500 篇），因此默认封顶为 4。

`serve` 增量重建消除了每次保存对全站 markdown 的 `O(N)` 重新解析：例如 50 篇文章的站点，编辑单篇时只重新解析 1 篇、复用其余 49 篇（`Partial rebuild: 1 changed, 49 reused`），站点规模越大、单文件越重，节省越明显。

---

## 验证

发布前执行：

```bash
go test ./... -race
make lint
git diff --check
```
