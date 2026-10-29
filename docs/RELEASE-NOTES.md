# SSHGuard v0.3.6

- fix: release Windows installer lock after staging cleanup
- release: prepare SSHGuard v0.3.5
- feat: add web setup and simplify platform and Agent uninstall
- release: prepare SSHGuard v0.3.4
- feat: simplify installer to a numbered menu and single port prompt
- docs: add paste-ready Baota reverse proxy configuration
- release: prepare SSHGuard v0.3.3
- fix: make direct installation domain-free and move proxy setup to documentation
- release: prepare SSHGuard v0.3.2
- feat: add Agent uninstall commands and restricted administrator-triggered updates
- fix: isolate API and Agent secrets under shared configuration directory
- feat: standalone web service and optional OpenResty-compatible deployment
- ci: add optional automatic versioning for one-click releases
- ci: validate a single build across deployment matrices before publishing
- release: prepare SSHGuard 0.3.1 with SQLite deployment support
- fix: isolate SQL cancellation from pooled Gin request contexts
- feat: support SQLite with secure deployment and permission regression tests
- docs: record full production candidate validation and refresh console preview
- fix: support MySQL 5.6 rule seeding and verify queue retention
- feat: add durable SSH collection, reliable alerting and realtime console
- test: start BusyBox syslog in the minimal Alpine fixture
- fix: support atomic replacement on PowerShell 5.1 and clear stale sessions
- fix: avoid validated PowerShell parameter reassignment after enrollment
- fix: handle PowerShell enrollment encoding and bootstrap TLS
- fix: harden update probes and Windows TLS test setup
- fix: isolate service accounts and verify Windows and Alpine agent upgrades
- feat: add one-command deployment and safe agent updates with MySQL 5.6 compatibility
- chore(deps): bump the npm-compatible-updates group
- chore(deps): bump the go-compatible-updates group with 3 updates
- chore: refine dependency updates and document cloud verification
- feat: publish SSHGuard preview with cross-platform agents and CI releases
- Initial commit

升级前备份数据库、配置和 ENCRYPTION_KEY。数据库类型不会自动切换或搬迁，程序回退不撤销数据库迁移。安装与升级见 [快速开始](QUICKSTART.md)。
