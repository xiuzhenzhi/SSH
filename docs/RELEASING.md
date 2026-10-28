# 构建、版本与 GitHub Actions

## 日常 CI

`.github/workflows/ci.yml` 在 main push、PR、workflow_dispatch 时运行，也被 Release 工作流复用：

1. Ubuntu/Windows：Go 单元测试和 vet；Linux 额外执行 race 测试；Windows 执行安装器 ACL helper 测试。
2. 两个独立服务容器：PostgreSQL 16、MySQL 8.4，执行完整真实数据库 API 集成测试。
3. 前端：npm ci、TypeScript/Vite 构建、high 级别依赖漏洞检查。
4. main、手动运行和 Release 的所有检查通过后，构建两个 Linux API 架构、四个 Agent，检查 ELF/PE 架构、静态链接和 SHA-256，上传产物。PR 只执行检查，不上传安装包。

流程没有生产凭证；CI 数据库密码只在临时测试容器内使用。不要把生产环境连接串放进 workflow。未通过检查不会发布安装产物。

Dependabot 按周分组提交 Go/npm 的 minor/patch 更新，不自动合并；major 升级单独评估兼容性。PR 检查失败不会影响已验证的 main/Release，也不能绕过检查直接合并。

## 产物

| 名称 | 用途 |
|---|---|
| sshguard-linux-amd64.tar.gz / sshguard-linux-arm64.tar.gz | 中央平台，包含 API、Web、全部 Agent 和文档 |
| sshguard-agent-linux-amd64 / linux-arm64 | Ubuntu/Debian/Alpine 共用静态二进制 |
| sshguard-agent-windows-amd64.exe / windows-arm64.exe | Windows 原生服务二进制 |
| install-agent.sh / install-agent.ps1 | 后台安装命令所下载的脚本 |
| uninstall-agent.sh / uninstall-agent.ps1 | 保留配置与日志的服务卸载脚本 |
| 单个 .sha256 / SHA256SUMS | 单文件及总清单完整性校验 |

Actions Artifact 是一次构建快照，保存 14 天；Release 是命名版本供部署使用。仓库私有时两者都需要访问权限。自动构建成功不代表已经在每个目标系统实机安装验证；ARM64 默认交叉编译。

## 创建预览版

1. 修改 `VERSION`，例如 `0.2.0-alpha.1`，长度不超过 32。
2. 更新 CHANGELOG、docs/RELEASE-NOTES.md 和验收记录。
3. 提交到 main，等待 CI 全部通过。
4. 在对应提交创建并推送标签：

```bash
git tag -a v0.2.0-alpha.1 -m "SSHGuard 0.2.0 alpha 1"
git push origin v0.2.0-alpha.1
```

Release 工作流检查标签与 VERSION 完全一致，再复用 CI、下载并校验产物，最后通过 GitHub 自带 Token 创建 Prerelease。所有当前版本都标为预览版；正式稳定版需在采集/通知闭环和生产验收完成后单独修改发布策略。

**不要移动已发布标签或覆盖同名二进制。** 修复使用新版本。发布任务因网络错误中断时先检查 Release 是否已经创建、附件是否完整；不要直接覆盖已有资产。不同源代码不得复用已发布的版本名。

## 本地复现

```bash
node scripts/build-release.mjs amd64
node scripts/build-release.mjs arm64 --reuse-web
node scripts/check-agent-artifacts.mjs
node scripts/package-release.mjs
```

输出为 dist/release。Agent 的 `-version` 和健康接口中的 version 可识别构建版本。VERSION 通过链接参数注入；源码直接 go run 使用代码内的开发默认值。

SHA-256 清单校验下载完整性，不是代码签名或独立的发布者身份认证。当前 Windows 程序未做 Authenticode 签名，按组织终端管理要求部署；不会关闭系统安全保护。

参考：[GitHub artifact](https://github.com/actions/upload-artifact)、[setup-go](https://github.com/actions/setup-go)。
