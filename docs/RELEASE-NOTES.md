# SSHGuard v0.3.16

- 仓库名称统一为 SSHGuard，更新安装下载、自动更新、Agent 产物复用及文档链接。
- 文档站迁移至 /SSHGuard/，首页使用现行文档路径，移除旧文档跳转和低于 v0.3.14 的历史内容。
- 移除早期 MySQL 迁移适配、旧 Windows 更新任务格式和目录式安装卸载兼容；保留 v0.3.14+ 管理入口与新安装必需的数据库迁移。

主控与 Agent 版本分别由 VERSION 与 AGENT_VERSION 管理。本次安装维护脚本发生变化，二者均为 0.3.16。仅支持 v0.3.14 及之后的升级路径。

升级前备份数据库、配置和 ENCRYPTION_KEY。参见 [安装与更新](guides/QUICKSTART.md)。
