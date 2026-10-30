# 构建与发布

## 工作流

| 工作流 | 用途 |
|---|---|
| CI | 按改动范围检查代码；勾选 `full_validation` 可运行完整安装矩阵 |
| 构建与发布 | 准备版本、完整验证并创建 Release |
| Documentation | 构建并发布文档站 |

## 从 Actions 发布

1. 打开 Actions →「构建与发布」→ **Run workflow**。
2. 选择分支，通常为 `main`。
3. 版本留空会自动递增补丁号，也可填写 `0.4.0` 或 `0.4.0-rc.1`。
4. 等待验证与发布完成，在 Releases 下载产物。

自动准备版本的 Git 提交署名使用 `xiuzhenzhi <xiuzhengzhi66@gmail.com>`；GitHub 推送操作账号仍由工作流凭据决定。

已有版本不能覆盖。工作流中断时使用 **Re-run failed jobs**，继续同一版本。

## 手动打标签

同步 `VERSION`、API / Agent 默认版本、安装器、安装示例和发布说明，提交并等待 CI 通过后执行：

```sh
git tag -a v0.3.14 -m "SSHGuard 0.3.14"
git push origin v0.3.14
```

标签须与 `VERSION` 一致。包含连字符的版本会发布为预发布版。

## 本地构建

```sh
node scripts/build/build-release.mjs amd64
node scripts/build/build-release.mjs arm64 --reuse-web --reuse-agents
node scripts/checks/check-agent-artifacts.mjs
node scripts/build/package-release.mjs
```

输出目录为 `dist/release`。`--reuse-agents` 要求先构建同版本 amd64 包。

## 发布产物

| 文件 | 内容 |
|---|---|
| `sshguard-linux-amd64.tar.gz` / `arm64` | 主控、Web、Agent、管理脚本和部署模板（不包含 docs） |
| `sshguard-agent-*` | Linux / Windows、amd64 / arm64 Agent |
| `install-agent.*` / `uninstall-agent.*` | Agent 安装与卸载脚本 |
| `sshguard.sh` | 统一安装、更新与服务管理入口 |
| `deploy.sh` / `quick-start.sh` / `update.sh` / `uninstall.sh` | 旧命令兼容入口，转交统一脚本 |
| `.sha256` / `SHA256SUMS` | 校验清单 |

完整发布流程包含跨平台测试、数据库测试和五组原生安装升级。验证结果见 [验证记录](VALIDATION.md)。
