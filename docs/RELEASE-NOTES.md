# SSHGuard v0.3.1

本版新增无需独立数据库服务的 SQLite 部署方式，并修复 SQLite 取消检查与请求对象复用之间的数据竞争。现有 PostgreSQL / MySQL 部署可保留原配置升级。

- SQLite 自动建表、WAL、外键检查和立即写事务；事件、告警和通知任务继续在同一事务中保存。
- 原生一键安装支持选择 `sqlite`，数据保存在 `/var/lib/sshguard-api/sshguard.db`；systemd 创建权限为 0700 的专用目录，升级保留数据与密钥。
- Compose 增加 `compose.sqlite.yaml` 和持久化卷，无需启动独立数据库容器。
- 数据库调用统一使用标准请求上下文，避免 Gin 对象池复用与驱动后台取消检查竞争，保留请求超时和取消行为。
- 收紧数据库目录权限，Docker 构建与源码包排除本地数据库和伴随文件；补充角色权限、Agent 身份隔离、迁移、持久化与并发事务测试。
- Linux CI 增加 SQLite 原生安装、升级、数据目录权限和 Agent 生命周期验证；README 补齐数据库选择、备份和全局角色权限边界。

升级前备份数据库和配置，保留 ENCRYPTION_KEY。已有 PostgreSQL / MySQL 不会自动切换到 SQLite，也不会自动搬迁数据。SQLite 适用于本机磁盘上的单实例部署；备份前停止 API，保留数据库及存在的 WAL/SHM 文件。先平台后 Agent；程序回退不撤销数据库迁移。一键入口默认版本为 v0.3.1，仅在 Release 附件发布完成后可用。

下载中央平台包 sshguard-linux-amd64.tar.gz / sshguard-linux-arm64.tar.gz，校验后解压执行 sudo sh deploy.sh。完整安装见 [README](../README.md)，生产分批验收与明确限制见 [生产验收](PRODUCTION.md)。
