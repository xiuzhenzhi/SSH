# 运维手册

## 上线前检查

- 先在 2–3 台非生产主机验证安装、重启恢复、采集、通知、断线恢复与撤销，再扩展到 10 台、最后覆盖全部服务器。
- 默认直连模式开放实际主控端口（默认 8080）；启用 HTTPS 反代后仅公开站点的 443（证书申请按需开放 80），同机 API 绑定 127.0.0.1。数据库仅允许 API 所在主机/隔离网络访问。
- 配置可信 HTTPS 证书和续期；操作系统启用时间同步。当前 API 接收最近 30 天的事件，未来时间最多容忍 5 分钟偏差。
- `.env`、api.env、数据库备份和 Agent 令牌均作为敏感配置保管，不放进 Git、Web 根目录、Issues 或日志截图。
- 没有完成采集/通知闭环前，不以本项目代替现有安全监控。

## 健康与状态

`GET /api/v1/health` 返回数据库连通性、数据库类型和版本；不是端到端日志采集健康检查。后台在线判定：最后心跳在 90 秒内，Agent 默认每 30 秒发送一次。

| 状态 | 含义 | 处理 |
|---|---|---|
| 待接入 | 创建资产后尚未注册 | 执行新生成的安装命令 |
| 在线 | 最近收到心跳 | 仅说明网络与身份验证正常，另看采集状态 |
| 离线 | 心跳超时 | 检查服务、出站网络、DNS、证书和平台健康 |
| 已撤销 | 管理员禁用身份 | 确认变更；需要恢复时重新生成命令注册 |

## 升级中央平台

向导管理的原生部署：

1. 阅读目标版本变更，备份数据库、`/etc/sshguard` 和 `/var/lib/sshguard-api/setup.json`（如有），记录当前版本。
2. 在独立测试环境验证新版迁移及基本读写。
3. 在主控执行 `sh /opt/sshguard/update.sh`，或在源码目录执行 `sh scripts/update.sh`；也可打开 `deploy.sh` 菜单选择「9 更新主控」。已安装时默认选「9」，「1」只用于安装。更新入口自动获取最新正式 Release 并校验，保留原配置和数据。
4. 检查日志及 health，验证登录、资产和一台测试 Agent。程序启动或健康检查失败会自动恢复旧程序，数据库迁移不会回退。

私有仓库先以运行更新脚本的用户执行 `gh auth login`；建议普通用户运行 `update.sh`，校验后才提权。可用 `SSHGUARD_VERSION=v目标版本 sh /opt/sshguard/update.sh` 指定版本；离线包从新包目录运行 `sudo sh deploy.sh --update`。详情见 [一键更新](QUICKSTART.md#更新中央平台)。手工部署不使用自动向导，继续在维护窗口替换 API、Web 与下载目录，并保留配置和证书。

旧目录没有新版更新器时，直接拉取仓库最新独立脚本并更新：

```sh
curl --proto '=https' --proto-redir '=https' --tlsv1.2 -fLsS https://raw.githubusercontent.com/xiuzhenzhi/SSH/main/scripts/quick-start.sh -o sshguard-update.sh && sh sshguard-update.sh --update
```

脚本从 `main` 更新，程序从正式 Release 更新；无需提前下载新版压缩包。更新后的文档位于 `/opt/sshguard/docs`，旧解压目录中的脚本和文档仍是原副本。详见 [拉取最新更新脚本](QUICKSTART.md#拉取最新更新脚本旧安装也可用)。

Compose：在同一项目目录更新源码，保留 `.env` 和命名卷，执行 `docker compose up -d --build`。不要使用 `down -v`。更换数据库类型是迁移项目，不是常规升级。

启动会自动执行未登记迁移。PostgreSQL 支持事务 DDL；MySQL 的 DDL 自动提交，建表迁移可重跑。**没有自动 down 迁移。** 数据库结构变更后的回滚须恢复经过验证的数据库备份与配置，并评估备份之后新增事件的恢复。

## 升级 Agent

先更新平台，再使用「服务器管理 → 更新命令」。更新保留身份、读取位置和本地队列，不重新注册。首次注册或重装才会轮换凭证。先在代表性系统验收，再分批推广；不能复制配置到其他机器。平台不提供远程 Shell 执行。

## 数据库备份与恢复演练

以下为原生部署示例，在安全的备份目录执行；密码由数据库工具交互输入，不放进命令参数。数据库端口仅限管理路径访问。

```bash
# PostgreSQL：自定义格式
pg_dump -h 127.0.0.1 -U sshguard -W -Fc -f sshguard-backup.dump sshguard
# 在数据库管理员创建的独立 sshguard_restore 库中恢复
pg_restore -h 127.0.0.1 -U sshguard -W --no-owner --no-acl -d sshguard_restore sshguard-backup.dump

# MySQL：当前项目没有存储过程/事件调度对象
mysqldump -h 127.0.0.1 -u sshguard --password --single-transaction --no-tablespaces --set-gtid-purged=OFF --result-file=sshguard-backup.sql sshguard
# 管理员先创建独立 sshguard_restore 库并授权，禁止直接覆盖生产库
mysql -h 127.0.0.1 -u sshguard --password sshguard_restore < sshguard-backup.sql
```

恢复后核对 schema_migrations、用户数、服务器数、事件时间范围和审计记录。在隔离环境验证登录/查询；不要让恢复演练平台向生产 Agent 分发注册命令。备份包含有效身份哈希和审计数据，应加密存储，并测试异地恢复；具体保留周期由运维制度确定。

## 会话、凭证与留存

管理员可撤销单台 Agent。Web 用户主动退出会删除会话，最长 12 小时到期。仅首次空用户表时使用 ADMIN_PASSWORD；改环境变量不会修改已存在的账号。

没有 MFA、SSO、用户禁用和角色编辑界面。事件、告警、审计默认保留 30 / 90 / 180 天，环境配置与清理边界见 [生产验收](PRODUCTION.md)。待投递和失败待处理通知保留。

渠道配置使用 ENCRYPTION_KEY 加密，必须与数据库共同备份。恢复演练平台不要启用外部通知，否则可能重复发送恢复出的待投递任务。隔离网络后核对任务状态，再决定是否启动 worker。当前 API 启动即运行 worker，演练环境必须先禁止外部出口或在离线恢复数据库中停用全部渠道。

## 采集与通知维护

原生向导部署/更新见 [快速开始](QUICKSTART.md)。Agent 更新保留身份，不需要重新注册。首次注册/撤销仍写审计日志；本机更新动作需从目标机器部署日志审计，平台通过心跳记录版本。

API 启动及每 10 分钟最多清理每表 1000 条过期会话、过期注册令牌和超过 24 小时的登录限速记录。另按配置保留期分批清理历史事件、审计及已完成通知；不自动删除服务器、待投递或失败待处理任务。MySQL 连接强制 UTC，不依赖数据库系统时区。

可在控制台修改自己的密码，全部旧会话立即失效；修改环境变量中的 ADMIN_PASSWORD 不会重置已存在用户密码。
