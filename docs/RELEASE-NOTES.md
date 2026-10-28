# SSHGuard v0.3.0

本版接通 SSH 采集、可靠上报、规则与通知的完整路径，重构控制台为浅色概览与告警工作台。

- Windows OpenSSH 事件、Linux journal / 文件采集，读取位置与本地队列原子持久化。
- HTTPS + WebSocket，匹配确认后出队、断线回退、事务去重；浏览器实时提示与轮询兜底。
- 登录 / 特权用户 / 失败阈值 / 离线 / 采集异常规则，可信 CIDR、分组、冷却、确认与恢复状态。
- 企业微信、钉钉、飞书、Telegram、SMTP、Webhook，AES-GCM 配置加密、持久化投递任务、限速、租约与重试。
- 保留原生一键部署、MySQL 5.6 兼容和四系统 Agent 安装更新；新增状态目录权限与服务读日志权限。

升级前备份数据库和配置。新增 004_alerting.sql；手工/Compose 部署必须生成并保存 ENCRYPTION_KEY（64 位十六进制）。原生向导仅在缺失时自动生成，后续更新保留。丢失密钥将无法恢复通知渠道配置。先平台后 Agent；程序回退不撤销数据库迁移。

下载中央平台包 sshguard-linux-amd64.tar.gz / sshguard-linux-arm64.tar.gz，校验后解压执行 sudo sh deploy.sh。完整安装见 [README](../README.md)，生产分批验收与明确限制见 [生产验收](PRODUCTION.md)。
