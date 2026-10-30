# SSHGuard v0.3.21

- 主控配置集中到 `/opt/SSHGuard/Server/api.env`，SQLite、初始化配置和运行数据统一放在 `Server/data/`，服务配置原件位于 `Server/sshguard-api.service`。不提供旧路径兼容或自动迁移，旧安装需卸载后全新安装。
- 更新仅替换已知程序文件，不移动或删除运行数据；失败时恢复程序和服务配置，成功后清理临时程序备份。卸载保留配置、数据和证书。
- 限制主控仅能写入数据目录，保护配置和程序文件；补充清理边界与配置、数据保持测试。
- 同步原生安装、备份恢复和反代文档。MySQL/PostgreSQL 外部数据库和 Docker 容器内部数据路径不变。

升级前备份数据库、配置和 ENCRYPTION_KEY。数据库类型不会自动切换或搬迁，程序回退不撤销数据库迁移。安装与升级见 [快速开始](guides/QUICKSTART.md)。
