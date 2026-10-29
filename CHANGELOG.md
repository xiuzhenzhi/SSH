# 更新日志

## 0.3.9

- 刷新统一到顶栏，退出改为箭头图标；侧栏按功能使用独立图标与悬停 / 点击动效，全站弹窗采用居中标题、统一表单和操作区，保留浅深色配色。
- Agent 安装与更新直接下载固定版本的 GitHub Release，校验 SHA-256 和版本号；已有节点需手动执行一次新版更新命令来迁移本地更新器。

- 新增 Cloudflare Turnstile、Google reCAPTCHA v2、极验第四代登录验证，服务端校验、主机绑定、时效和重放保护；密钥加密且不回显。
- 登录来源 IP 在滚动 60 秒内错误 5 次封禁 24 小时，跨重启 / 实例保留，管理员可解除；修复清理任务并发误删新封禁的竞态。

- 服务器列表改为紧凑行，增加可复制的 IPv4 / IPv6，维护操作收进菜单；移除重复标题、宣传文案、统计卡片脚注和页脚。
- 安装 / 更新窗口改为系统标签、上报设置与复制命令；右上角只显示一个 Admin，审计动作与角色显示中文，SSH 事件补齐来源端口。
- 增加独立 IP 白名单，支持 IP / CIDR、IPv4 / IPv6；仅免除 SSH 事件告警，保留原始事件，变更受角色、同源校验和审计保护。
- 安装时可选择 1–30 秒上报，默认 5 秒；安装凭证绑定间隔并写入 Agent 配置，运行期不接收配置变更，更新保留原值。
- 修复 Alpine 标准 BusyBox syslog 的采集权限：专用 root:sshguard 认证日志和 setgid 目录，Agent 不加入 wheel；同版本更新也修复权限。
- 总览图表使用 ECharts，支持面积曲线、过渡动画、悬停 / 键盘明细和 24 小时 / 7 / 14 / 30 天；增加全站深色模式和持久化切换按钮。
- 审计增加来源 IP 与 UA，只信任显式配置的代理（默认仅本机回环）；兼容历史空值。
- 总览统计共享 5 秒短缓存，页面刷新合并；无变化文件不落盘，地址不重复更新，心跳错误日志限频，保留断线队列与资源回收验证。
- Windows 生成的安装 / 更新 / 卸载命令遵循当前 PowerShell 执行策略，取消主动指定 Bypass；本地预览直接启动编译程序。

## 0.3.8

- Agent 的 journalctl 诊断输出限制为 4 KiB，继续排空超出部分；超长记录会立即终止日志生产进程再等待退出。
- 日志轮转目录每批读取 128 个条目，检查取消状态，不再把整个目录一次性加载到内存。
- 增加 300 次真实断线重连的堆 / goroutine 回收检查，以及 20 次启动取消后的队列句柄释放检查。
- 修复 Linux 原生 CI 的反代测试网页路径：优先使用发行包内的 web 目录，避免误判 CSP 缺失。

## 0.3.7（仅标签，未发布）

- 主控默认使用 IP + 端口和 SQLite；网页只需创建管理员，完成后自动登录并进入服务器管理。
- 打通无需域名的 Agent 安装、HTTP / WebSocket 上报、手动与平台触发更新；HTTP 显式授权随本地配置保留，HTTPS 继续严格验证证书。
- 安装菜单新增初始化链接恢复入口，支持 `--port`，状态页显示可访问地址；本地管理和卸载不再重复下载发行包。
- 增加 Windows / Linux 真实进程直连验证，扩展原生 CI 覆盖无域名 Agent 安装与升级。
- 统一主控与 Agent 的地址校验，拒绝异常端口、空查询串和片段；PUBLIC_URL 规范化后与浏览器 Origin 保持一致。
- 初始化只接受单个 JSON 对象；响应丢失时查询完成状态，避免重复初始化。
- IP 直连 HTTP 页面缺少 Clipboard API 时，安装、更新和卸载命令提供点击复制回退。
- 统一 Nginx / OpenResty / 宝塔反代示例，原生模式反代完整应用，延长初始化超时、禁用代理缓存并验证 WebSocket；自定义端口增加手动配置提示。
- 修复预览构建版本覆盖后 VERSION 清单与程序不一致的问题；安装前验证显式证书输入。

## 0.3.6

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

## 0.3.5

- 新安装仅询问端口，首次访问网页选择 SQLite/MySQL/PostgreSQL、测试连接并创建管理员；初始化口令与同源校验防止未授权抢占，完成后关闭初始化入口。
- 兼容旧版中央平台的独立卸载脚本与菜单，停止服务、移除当前程序、保留数据库和配置，不操作现有 Web 服务或 Agent。
- Agent 自动识别 Linux 服务管理器并补充依赖，安装后保存 Linux/Windows 离线卸载脚本；保留平台触发固定版本更新，不提供其他远程命令。

## 0.3.4

- 对齐简洁安装体验：统一编号管理菜单，默认 SQLite，只询问应用端口；数据库和反代配置不进入默认安装问答。
- 首次交互安装直接显示检测到的访问地址、管理员账号和初始密码；重定向输出与 CI 日志不打印密码。
- 一键入口支持安装/更新、状态、日志、启动、停止、重启，安装包本身也可直接打开同样的管理菜单。

## 0.3.3

- 默认安装只询问端口和数据库，取消域名必填和自动选择已有反向代理；安装结果简化为中文访问及账户提示。
- 直连 HTTP 支持按实际访问地址进行同源校验，不信任转发头；固定 HTTPS 地址继续严格匹配 PUBLIC_URL。
- 反代和 WebSocket 配置移入可选部署文档；增加已有代理环境下无域名安装、不同 IP/主机地址登录及升级的 CI 验证。

## 0.3.2

- 新增各平台卸载命令和管理员触发的受限 Agent 更新；独立维护服务仅执行固定本地更新流程，保留低权限采集，无远程 shell 或远程卸载。
- 更新任务绑定服务器与凭证，防止跨机器领取、重复执行；增加权限、审计、Windows/Linux 安装与更新回归检查。
- 中央程序直接提供网页、API 和 Agent 下载，支持内置 HTTPS，无需另装 Nginx；静态目录限制文件范围和符号链接越界。
- 安装向导默认 SQLite，增加独立、已有 Nginx / OpenResty / 宝塔、托管 Nginx 三种模式，仅补装缺少的依赖。
- 增加中文管理菜单、状态、日志和重启命令；升级保留数据库、密钥和接入方式，发布目录仅复制发行内容。
- CI 增加独立 HTTPS 和已有代理的安装、升级与恢复验证，检查原站点不被覆盖、非托管模式不调用 Nginx 或包管理器。
- 整理 CI 与构建发布流程，支持手动指定版本或自动递增补丁版本，构建产物通过部署测试后发布。

## 0.3.1

- 修复 SQLite 取消检查与 Gin 请求对象复用之间的数据竞争；数据库调用统一使用标准请求上下文，保留请求取消与超时。

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
