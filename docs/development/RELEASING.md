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

同步主控 `VERSION`、API 默认版本、安装示例和发布说明；仅 Agent 输入变化时更新 `AGENT_VERSION`、API 的 `AgentVersion`、Agent 默认版本和安装器，提交并等待 CI 通过后执行：

```sh
git tag -a v0.3.16 -m "SSHGuard 0.3.16"
git push origin v0.3.16
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
| `deploy.sh` / `quick-start.sh` / `update.sh` / `uninstall.sh` | v0.3.14+ 发行包管理入口，转交统一脚本 |
| `.sha256` / `SHA256SUMS` | 校验清单 |

完整发布流程包含跨平台测试、数据库测试和五组原生安装升级。验证结果见 [验证记录](VALIDATION.md)。

## 独立版本与按组件验证

主控版本由 `VERSION` 管理，Agent 版本由 `AGENT_VERSION` 管理，两者允许不同。自动准备版本比较上次 Agent 发布标签与当前代码：Agent 代码、共享网络工具、Go 依赖、安装/卸载脚本或 Agent 构建逻辑发生变化才更新 Agent 版本。缺失历史时采取完整构建。纯主控、前端或文档变化不会推进 Agent 版本。

主控构建读取两个版本；安装链接、远程更新目标和「已是当前版本」判断使用 Agent 版本。主控与 Agent 版本不同时，打包从既有 Agent Release 下载四个平台产物并验证校验和与架构，不重复交叉编译。下载或校验失败会中止发布，不会静默替换产物。本地复用需要 Git 标签历史及 `gh`。Web Docker 镜像仅构建前端，Agent 官方安装命令直接访问其 GitHub Release。

| 变化范围 | 常规 CI |
|---|---|
| 文档 | 文档链接检查和文档站；不启动数据库或应用构建 |
| 前端 | 前端测试和构建；构建产物按源码缓存 |
| Agent | Agent 与共享网络包测试；不启动 MySQL/PostgreSQL |
| 主控 API、SQL、存储、迁移 | 主控测试和数据库兼容矩阵 |
| 通知、GeoIP | 主控相关测试；不单独启动外部数据库 |
| 共享依赖、工作流、无法识别的路径 | 保守执行全部检查 |

Release 按上个标签到当前提交判断变化，仍构建并校验主控发布包。安装部署矩阵只在主控、数据库或部署相关变化时执行。手动 CI 的 `full_validation` 保留完整验证能力。数据库结构继续使用已有迁移编号，不随主控版本制造空迁移。

文档站只监听内容、配置和文档构建依赖；单独变更 `VERSION` 不再触发文档部署。真实发布说明变化仍会正常重建文档。
