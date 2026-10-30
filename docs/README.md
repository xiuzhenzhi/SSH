# SSHGuard 文档

当前版本以根目录 [VERSION](../VERSION) 和 [正式 Release](https://github.com/xiuzhenzhi/SSH/releases/latest) 为准；本次文档核对日期为 2026-10-30。更新日志和验证记录保留历史版本，不能视为当前版本的验收结果。

## 安装、更新与日常管理

| 需要做什么 | 文档 |
|---|---|
| 安装主控、网页初始化、菜单管理、一键更新、拉取最新更新脚本 | [快速开始](QUICKSTART.md) |
| 理解无域名默认安装与接入流程 | [安装流程](INSTALLATION-FLOW.md) |
| 手工部署二进制与 systemd | [原生部署](INSTALL-NATIVE.md) |
| Docker Compose 安装与更新 | [Compose 部署](INSTALL-COMPOSE.md) |
| Agent 安装、固定版本更新、卸载和状态迁移 | [Agent 管理](AGENTS.md) |
| 宝塔、Nginx、OpenResty、HTTPS 与端口迁移 | [反代与 HTTPS](REVERSE-PROXY.md) |
| 主控升级、备份、恢复和运行维护 | [运维手册](OPERATIONS.md) |
| 安装、更新、登录、采集或下载失败 | [故障排查](TROUBLESHOOTING.md) |

原生菜单「1」仅安装，「9」下载最新正式版并更新；已有安装回车默认选择「9」。旧安装没有新菜单时，从仓库拉取自包含更新脚本，不必先下载发行包：

```sh
curl --proto '=https' --proto-redir '=https' --tlsv1.2 -fLsS https://raw.githubusercontent.com/xiuzhenzhi/SSH/main/scripts/quick-start.sh -o sshguard-update.sh && sh sshguard-update.sh --update
```

脚本来自 `main`，二进制来自正式 Release。原生更新保留数据库、管理员、端口、Agent 身份和加密密钥；Compose 使用自己的源码构建流程。详细权限、指定版本和离线方式见快速开始。

## 功能与安全

| 主题 | 文档 |
|---|---|
| 告警规则、单条 / 批量确认、通知渠道和投递 | [告警与通知](ALERTING.md) |
| 服务器与事件国家、认证失败来源国家 TOP 5、公网地址 | [地理位置](GEOIP.md) |
| 验证码、登录来源封禁和本机恢复 | [登录防护](LOGIN-SECURITY.md) |
| 部署检查、分批验收、容量及已知边界 | [生产验收](PRODUCTION.md) |
| 漏洞反馈、身份、权限与下载信任 | [安全策略](../SECURITY.md) |

控制台各菜单使用独立路径，刷新保留当前菜单及分页 / 筛选；反代须将整个站点交给主控处理，不能只代理 API。

## 开发、发布与证据

- [开发与测试](DEVELOPMENT.md)：本地运行、构建、数据库测试及脚本回归。
- [API 参考](API.md) 与 [传输协议](TRANSPORT.md)：鉴权、请求格式、实时连接和数据确认。
- [构建与发布](RELEASING.md)：版本准备、不可变标签、完整 CI 和产物。
- [本版发布说明](RELEASE-NOTES.md)、[更新日志](../CHANGELOG.md) 与 [验证记录](VALIDATION.md)。
- [参与开发](../CONTRIBUTING.md) 与 [产品范围](ROADMAP.md)。
