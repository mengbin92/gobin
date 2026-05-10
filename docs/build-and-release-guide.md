# Gobin 跨平台构建和发布指南

本文档介绍如何为 Gobin 构建不同平台的二进制文件并发布到 GitHub Release。

---

## 目录

1. [方法一：使用 GitHub Actions（推荐）](#方法一使用-github-actions推荐)
2. [方法二：手动构建（本地脚本）](#方法二手动构建本地脚本)
3. [方法三：使用 GitHub CLI（手动上传）](#方法三使用-github-cli手动上传)
4. [支持的构建目标](#支持的构建目标)
5. [检查清单](#检查清单)

---

## 方法一：使用 GitHub Actions（推荐）

这是最推荐的自动化方法。每次你推送一个版本标签时，GitHub Actions 会自动构建所有平台的二进制文件并创建 Release。

### 步骤

#### 1. 确保工作流文件存在

检查 `.github/workflows/release.yml` 文件是否存在：

```bash
ls .github/workflows/release.yml
```

#### 2. 推送版本标签

```bash
git tag v1.0.0
git push origin v1.0.0
```

#### 3. 等待构建完成

GitHub Actions 会自动开始构建：

1. 访问 https://github.com/mengbin92/gobin/actions
2. 查看 `Build and Release` 工作流的运行状态
3. 等待所有平台的构建完成（通常 5-10 分钟）

#### 4. 查看 Release

构建完成后，访问 https://github.com/mengbin92/gobin/releases 查看自动创建的 Release。

### 自定义构建内容

编辑 `.github/workflows/release.yml` 中的 `matrix` 部分，添加或删除构建目标：

```yaml
matrix:
  include:
    - goos: linux
      goarch: amd64
      suffix: linux-amd64
    - goos: linux
      goarch: arm64
      suffix: linux-arm64
    # ... 更多目标
```

---

## 方法二：手动构建（本地脚本）

如果你想在本地构建所有二进制文件，可以使用提供的构建脚本。

### 步骤

#### 1. 运行构建脚本

```bash
# 在项目根目录
cd /Users/mac/vscode/mengbin/gobin

# 运行构建脚本
./scripts/build-all.sh
```

#### 2. 查看构建结果

构建完成后，所有二进制文件位于 `./dist` 目录：

```bash
ls -lh dist/
```

输出示例：
```
-rwxr-xr-x 1 user user 8.5M Jan 7 10:30 gobin-v1.0.0-linux-amd64
-rwxr-xr-x 1 user user 8.3M Jan 7 10:30 gobin-v1.0.0-linux-arm64
-rwxr-xr-x 1 user user 8.1M Jan 7 10:30 gobin-v1.0.0-darwin-amd64
-rwxr-xr-x 1 user user 8.2M Jan 7 10:30 gobin-v1.0.0-darwin-arm64
-rwxr-xr-x 1 user user 8.4M Jan 7 10:30 gobin-v1.0.0-windows-amd64.exe
-rwxr-xr-x 1 user user 8.6M Jan 7 10:30 gobin-v1.0.0-freebsd-amd64
```

#### 3. 上传到 GitHub Release

##### 选项 A：使用 GitHub CLI（推荐）

安装 GitHub CLI（如果尚未安装）：

```bash
# macOS (使用 Homebrew)
brew install gh

# 或 Ubuntu/Debian
sudo apt install gh

# 登录
gh auth login
```

创建 Release：

```bash
# 创建 Release
gh release create v1.0.0 \
  --title "Gobin v1.0.0" \
  --notes-file RELEASE-NOTES-v1.0.md \
  dist/*
```

发布说明文件建议按 tag 命名，例如 `RELEASE-NOTES-v1.1.0.md`。自动发布工作流会按顺序查找：

- `RELEASE-NOTES-v1.1.0.md`
- `RELEASE-NOTES-1.1.0.md`
- `RELEASE-NOTES.md`

##### 选项 B：通过 GitHub Web 界面

1. 访问 https://github.com/mengbin92/gobin/releases
2. 点击 "Draft a new release"
3. 选择或创建一个 tag（如 v1.0.0）
4. 填写标题和描述（可以复制 RELEASE-NOTES-v1.0.md 的内容）
5. 将构建好的二进制文件拖到上传区域
6. 点击 "Publish release"

---

## 方法三：使用 GitHub CLI（手动上传）

如果你已经创建了 Release，但想补充上传二进制文件：

### 步骤

#### 1. 安装并配置 GitHub CLI

```bash
# 安装（macOS）
brew install gh

# 登录
gh auth login
```

#### 2. 使用上传脚本

```bash
# 构建二进制文件（如果还没构建）
./scripts/build-all.sh

# 运行上传脚本
./scripts/upload-release.sh v1.0.0
```

#### 3. 或手动上传

```bash
# 上传单个文件
gh release upload v1.0.0 dist/gobin-v1.0.0-linux-amd64

# 上传所有文件
gh release upload v1.0.0 dist/*
```

---

## 支持的构建目标

### 官方支持

| 操作系统 | 架构 | 文件名格式 | 备注 |
|---------|------|-----------|-----|
| Linux   | amd64 | `gobin-{version}-linux-amd64` | 最常用的 Linux 构建 |
| Linux   | arm64 | `gobin-{version}-linux-arm64` | ARM64 Linux (如 Raspberry Pi, AWS Graviton) |
| macOS   | amd64 | `gobin-{version}-darwin-amd64` | Intel Mac |
| macOS   | arm64 | `gobin-{version}-darwin-arm64` | Apple Silicon Mac (M1/M2/M3) |
| Windows | amd64 | `gobin-{version}-windows-amd64.exe` | Windows 64-bit |
| FreeBSD | amd64 | `gobin-{version}-freebsd-amd64` | FreeBSD |

### 可选支持（可添加）

| 操作系统 | 架构 | 文件名格式 |
|---------|------|-----------|
| Linux   | 386 | `gobin-{version}-linux-386` |
| Linux   | arm | `gobin-{version}-linux-armv7` |
| Windows | 386 | `gobin-{version}-windows-386.exe` |
| OpenBSD | amd64 | `gobin-{version}-openbsd-amd64` |
| NetBSD  | amd64 | `gobin-{version}-netbsd-amd64` |
| Solaris | amd64 | `gobin-{version}-solaris-amd64` |

---

## 检查清单

### 发布前检查

- [ ] 所有测试通过: `go test ./...`
- [ ] 代码已提交并推送到远程
- [ ] 创建了版本标签: `git tag v1.0.0`
- [ ] 编写了发布说明（例如 `RELEASE-NOTES-v1.0.0.md`）
- [ ] 更新了 CHANGELOG

### 发布时检查

- [ ] 所有平台的二进制文件已成功构建
- [ ] 二进制文件已上传到 GitHub Release
- [ ] Release 包含完整的说明文档
- [ ] 示例站点可用且更新到最新版本

### 发布后检查

- [ ] 发布了 GitHub Release
- [ ] 二进制文件可以正常下载和使用
- [ ] 更新了文档中的版本引用
- [ ] 在社区发布了公告（可选）

---

## 故障排除

### 构建失败

**问题**: `exec: "go": executable file not found in $PATH`

**解决**: 确保 Go 已安装并配置在 PATH 中：
```bash
go version
```

### GitHub CLI 认证失败

**问题**: `gh: Not logged in`

**解决**: 运行 `gh auth login` 并按照提示进行认证。

### 上传文件失败

**问题**: `failed to upload release asset: already exists`

**解决**: 文件已存在，删除后重试：
```bash
gh release delete-asset v1.0.0 filename
gh release upload v1.0.0 filename
```

### Windows 构建无扩展名

**问题**: Windows 二进制文件没有 `.exe` 扩展名

**解决**: 确保在构建脚本中指定 `.exe` 扩展名。

---

## 最佳实践

### 1. 使用语义化版本

遵循 [Semantic Versioning](https://semver.org/) 规范：

- `v1.0.0` - 主要版本
- `v1.1.0` - 次要版本（新功能，向后兼容）
- `v1.1.1` - 补丁版本（bug 修复）

### 2. 包含版本信息

在构建时注入版本信息：

```bash
go build \
  -ldflags "-X main.Version=${VERSION} -X main.Commit=${COMMIT}" \
  -o binary \
  ./cmd/app
```

### 3. 压缩二进制文件

对于正式发布，可以考虑压缩二进制文件：

```bash
# 使用 upx 压缩（可选）
upx --best dist/gobin-v1.0.0-linux-amd64
```

### 4. 提供 checksums

生成校验文件：

```bash
cd dist
sha256sum gobin-* > checksums.txt
```

### 5. 自动化一切

尽可能使用 GitHub Actions 自动化：
- 自动构建
- 自动测试
- 自动发布

---

## 相关文件

- `.github/workflows/release.yml` - GitHub Actions 工作流
- `scripts/build-all.sh` - 本地构建脚本
- `scripts/upload-release.sh` - 上传脚本
- `RELEASE-NOTES-v1.0.md` - 发布说明模板

---

## 更多信息

- [GoReleaser](https://goreleaser.com/) - 更高级的 Go 发布工具
- [GitHub Actions](https://docs.github.com/en/actions) - GitHub Actions 文档
- [GitHub CLI](https://cli.github.com/) - GitHub CLI 文档

---

*文档版本: v1.0.0*
*更新日期: 2026-01-07*
