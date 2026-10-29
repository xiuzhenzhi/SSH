# SSHGuard v0.3.2

- feat: add Agent uninstall commands and restricted administrator-triggered updates
- fix: isolate API and Agent secrets under shared configuration directory
- feat: standalone web service and optional OpenResty-compatible deployment
- ci: add optional automatic versioning for one-click releases
- ci: validate a single build across deployment matrices before publishing

升级前备份数据库、配置和 ENCRYPTION_KEY。数据库类型不会自动切换或搬迁，程序回退不撤销数据库迁移。安装与升级见 [快速开始](QUICKSTART.md)。
