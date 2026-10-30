# 运维手册

> 从 v0.3.20 起使用新目录布局，旧版不自动迁移，请先使用旧版脚本卸载后再安装新版本。v0.3.19 用户请查阅 [对应版本文档](https://github.com/xiuzhenzhi/SSHGuard/tree/v0.3.19/docs)。

## 状态与日志

```sh
sudo sh /opt/SSHGuard/Server/sshguard.sh --status
sudo sh /opt/SSHGuard/Server/sshguard.sh --logs
sudo sh /opt/SSHGuard/Server/sshguard.sh --restart
```

| 控制台状态 | 处理 |
|---|---|
| 待接入 | 执行安装命令 |
| 在线 | 查看采集状态，验证 SSH 事件 |
| 离线 | 检查服务、网络与日志 |
| 已撤销 | 需要恢复时重新生成安装命令 |

## 升级中央平台

```sh
sh /opt/SSHGuard/Server/sshguard.sh --update
```

旧安装使用 [拉取最新更新脚本](../guides/QUICKSTART.md#更新中央平台)。Compose 按 [Compose 更新](INSTALL-COMPOSE.md#更新) 操作。

更新前备份，更新后检查登录、服务器状态和事件。程序启动失败会回退旧程序；数据库恢复需使用备份。

## 升级 Agent

在「服务器管理」选择「更新命令」或「平台更新」，操作见 [Agent 管理](../guides/AGENTS.md#更新)。

## 备份与恢复

同时保存数据库、配置和 `ENCRYPTION_KEY`。

| 部署方式 | 需要备份 |
|---|---|
| 原生 | `/etc/sshguard/`、`/var/lib/sshguard-api/` |
| Compose | `.env`、数据库命名卷、证书 |

SQLite 备份前停止 API，备份整个数据目录后再启动。

PostgreSQL：

```sh
pg_dump -h 127.0.0.1 -U sshguard -W -Fc -f sshguard-backup.dump sshguard
# 恢复到预先创建的测试库
pg_restore -h 127.0.0.1 -U sshguard -W --no-owner --no-acl -d sshguard_restore sshguard-backup.dump
```

MySQL：

```sh
mysqldump -h 127.0.0.1 -u sshguard --password --single-transaction --no-tablespaces --set-gtid-purged=OFF --result-file=sshguard-backup.sql sshguard
mysql -h 127.0.0.1 -u sshguard --password sshguard_restore < sshguard-backup.sql
```

恢复后核对用户、服务器和事件。测试恢复时停用通知渠道，避免重复发送。

## 数据保留

| 配置 | 默认 |
|---|---|
| `EVENT_RETENTION_DAYS` | 事件 30 天 |
| `ALERT_RETENTION_DAYS` | 已完成告警与投递 90 天 |
| `AUDIT_RETENTION_DAYS` | 审计 180 天 |

安装环境变量范围为 30–3650 天。管理员可在「数据库」页面设置 0–3650 天，0 表示永久保留，网页设置优先于环境变量。每 10 分钟分批清理，待投递和失败待处理任务不自动删除。详情见[数据库与数据保留](../guides/DATABASE.md)。

## 账号与密钥

在控制台修改密码，修改后需重新登录。`ADMIN_PASSWORD` 仅用于初始化，不会重置已有账号。

更新时保留 `ENCRYPTION_KEY`；丢失密钥后无法解密通知渠道和其他已保存凭证。
