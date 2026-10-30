# SSHGuard v0.3.20

- Linux 主控固定安装到 `/opt/SSHGuard/Server/`，Agent 程序、配置、运行数据和管理脚本集中到 `/opt/SSHGuard/Agent/`；Windows Agent 集中到 `%ProgramData%\SSHGuard\Agent\`。
- **旧目录不支持原地升级或自动迁移。请备份需要保留的数据，使用旧版卸载脚本卸载后重新安装。Agent 卸载会清除本机配置与待传数据，需重新接入。**
- 同一新布局内更新保留程序校验、失败恢复和健康检查后清理，主控与 Agent 清理互不影响。
- 发布主控与前端的 amd64/arm64 Docker 镜像；仅发布版本时构建镜像，普通推送和手动完整 CI 不构建镜像。
- 补齐目录隔离、卸载边界、启动失败恢复及 Windows 权限验证，同步安装文档。

升级前备份数据库、配置和 ENCRYPTION_KEY。数据库类型不会自动切换或搬迁，程序回退不撤销数据库迁移。安装与升级见 [快速开始](guides/QUICKSTART.md)。
