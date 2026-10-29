# 更新日志

## 未发布

- 新增 SQLite 数据库支持：自动迁移、外键检查、WAL、立即写事务及时间统计兼容，无需单独安装数据库服务。
- 新增 SQLite Compose 持久化配置、原生安装向导选项及 systemd 私有数据目录；升级保留数据库与加密密钥。
- 收紧容器数据库目录权限，源码归档与 Docker 构建排除数据库及伴随文件，避免意外分发业务数据。
- 数据库集成测试默认使用临时 SQLite 文件，补充并发事务、重复迁移、重开持久化、角色权限和 Agent 身份隔离验证。
- Linux 原生部署 CI 增加 SQLite 安装、更新和数据目录权限检查；更新数据库选择、部署、备份及产品权限边界说明。

## 0.3.0

- 真实 Windows / Linux SSH 采集、持久化游标与有界队列。
- HTTPS + WebSocket 确认、断线重传、跨通道事务去重和浏览器实时刷新。
- 登录、失败阈值、特权账号、离线与采集异常规则；六类通知、加密配置、租约重试、确认告警。
- 重构浅色概览、趋势、健康和通知管理，增加减少动态效果支持。
- 更新采集服务权限、状态目录、旧 MySQL 字符集迁移兼容与保留期清理。

历史版本记录：


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
