# SSHGuard v0.3.19

- fix: prune old controller releases after healthy updates
- docs: record verified v0.3.18 release
- fix: retain CI coverage across cancelled pushes
- test: verify component combinations using real Git history

升级前备份数据库、配置和 ENCRYPTION_KEY。数据库类型不会自动切换或搬迁，程序回退不撤销数据库迁移。安装与升级见 [快速开始](guides/QUICKSTART.md)。
