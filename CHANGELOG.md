# 更新日志

## 0.3.0-alpha.1

- 原生一键部署/更新向导、固定版本下载校验、可选自动 HTTPS、失败切回旧程序。
- Windows/Linux Agent 保留凭证更新、并发安装锁、校验和检查、服务启动检查和失败恢复。
- MySQL 5.6.51 兼容迁移、统一 utf8mb4/UTC、重试索引迁移、真实数据库 CI 矩阵。
- 自助改密并失效所有会话；修复改密与登录并发签发旧凭证的竞争。
- API 请求取消/超时、全局认证预算、只读 Agent 凭证、分批清理过期认证数据。
- Vue 组件按需加载、取消过期请求、减少轮询和统计查询、显示实际版本。
- 仍未实现 SSH 日志采集和通知，保持明确的 alpha 功能边界。

## 0.2.0-alpha.1

### 基础平台

- Go/Gin API，Vue 3/TypeScript/Element Plus 后台。
- bcrypt 登录、数据库会话、Origin 校验、基础 RBAC、审计事务。
- MySQL/PostgreSQL 独立迁移；原生 systemd/Nginx 与可选 Compose 部署。
- 服务器管理、一次性注册、凭证撤销、心跳、事件批量上报与幂等查询。

### 跨平台 Agent

- Windows 原生 SCM 服务及 LocalService 文件 ACL。
- Ubuntu/Debian systemd 和 Alpine OpenRC，POSIX sh 安装脚本。
- Windows/Linux × amd64/arm64 构建、SHA-256、后台按系统生成命令。

### 工程与交付

- 双数据库集成测试、Windows 服务 handler 与安装器 ACL 测试。
- GitHub Actions 测试、打包和标签 Prerelease；安装、运维、排障及路线图文档。

### 已知边界

真实 SSH 日志采集、持久化补传队列、规则引擎和外部通知尚未实现；未完成 70 台生产负载验收。
