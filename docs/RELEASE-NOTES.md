# SSHGuard v0.3.0-alpha.1

本版重点：让中央平台和 Agent 更容易安装、更新和维护。

- 原生一键部署，无需 Docker/Go/Node：本机 PostgreSQL 或已有 MySQL/PostgreSQL、systemd、Nginx、可选自动 HTTPS。
- Agent 支持 Windows / Ubuntu / Debian / Alpine，amd64 / arm64；保留凭证更新、SHA256 校验和失败回退。
- MySQL 5.6.51 / 8.4 兼容与测试，中文/emoji、UTC、索引迁移重试。
- 改密后统一失效旧会话、请求取消/超时、认证限流、只读 Agent 凭证、认证元数据清理。
- Web 按需加载、动态页面 30 秒刷新、版本展示、更新命令入口。

升级前备份数据库与配置，新增索引会执行 DDL；自动回退仅恢复程序，不逆向迁移数据库。从 v0.2 手工部署迁入步骤见 [一键部署文档](QUICKSTART.md)。先更新平台，再更新 Agent。

下载中央平台包 sshguard-linux-amd64.tar.gz / sshguard-linux-arm64.tar.gz，校验后解压，运行 sudo sh deploy.sh。Windows/Linux 独立 Agent 和安装器同时提供。安装说明见仓库 README。

这仍是基础预览版：Agent 只有注册和心跳，真实 SSH 日志采集、规则、通知尚未实现。MySQL 5.6 已停止官方维护，兼容性不代表数据库安全维护。无需把 GitHub 凭证分发到 Agent。
