# SSHGuard 0.2.0-alpha.1

这是基础预览版本，适合小规模测试和接口联调，不是完整生产告警系统。

已提供：

- Go API + Vue 后台，登录、角色权限、服务器管理、事件查询和操作审计。
- MySQL / PostgreSQL 二选一；原生部署 / Docker Compose 二选一。
- Windows、Ubuntu、Debian、Alpine Agent 注册与心跳；Windows/Linux 各 amd64/arm64 构建。
- 后台生成 15 分钟单次安装命令、TLS 校验、凭证撤销、事件去重 API。
- Linux/Windows 单元测试、双数据库集成测试与自动打包。

尚未提供：真实 SSH 日志采集、断线补传队列、规则引擎、Telegram/Webhook/SMTP 投递、MFA，以及 70 台生产容量验收。

下载前请按中央平台 CPU 架构选择 sshguard-linux-*.tar.gz，按 SHA256SUMS 校验后阅读随包安装文档。Agent 由管理后台通过自己的 HTTPS 域名分发；无需把 GitHub 凭证复制到被管理主机。
