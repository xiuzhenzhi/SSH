# 验证记录

## v0.3 开发验收（2026-10-28）

- Go 单元测试与 vet 通过；四种 Agent 平台/架构交叉编译通过。
- PostgreSQL 16、本地 MySQL 8.4.13：真实迁移、认证/权限、事件/告警事务去重、WSS 跨 HTTPS 重放、长连接凭证撤销、通知配置遮蔽、任务租约/CAS/重试均通过。MySQL 旧 utf8mb4_bin 表迁移后外键可正常建立。
- 70 个并发 Agent WSS 连接，每个上报 10 条事件后通过 HTTPS 重传：最终 700 条唯一事件、70 条冷却后告警。PostgreSQL 本机约 0.79 秒，MySQL 本机约 0.20 秒，仅为隔离 fixture 的处理耗时，不含真实采集、TLS、通知服务商或长期稳定性，不是生产容量保证。
- Agent 测试覆盖队列重启、游标原子保存、满队列不推进、确认丢失后 HTTPS 回退、拒绝错误 ACK、401/503 保留队列、无效记录隔离、文件轮转/半行、Windows 本机事件日志 XML/Unicode 解码。
- 通知测试覆盖配置密文认证与渠道绑定、私网目标阻止、TLS 不受信任拒绝、服务商应用层错误/缺失 ACK、重定向拒绝、SMTP TLS/STARTTLS、明文拒绝及 DATA 已接受但 QUIT 丢失。
- 浏览器实测管理员登录、概览实时连接、渠道类型表单、390px 手机布局；页面无整体横向溢出，表格/导航可横向滚动。截图为独立测试库。
- Vue 类型检查和生产构建通过；JS 约 454 kB（gzip 158 kB），CSS 约 124 kB（gzip 19 kB）；未引入大型图表库。
- npm audit 0 漏洞。govulncheck：0 可达代码漏洞、0 导入包漏洞；仍有 1 个未调用依赖模块提示，持续跟进。

新增功能的最终云端 CI 结果在发布前补充。此前安装升级基线完整 CI 已通过：[40668911927](https://github.com/xiuzhenzhi/SSH/actions/runs/40668911927)，包含 Windows SCM、Ubuntu systemd/Nginx、Alpine OpenRC、MySQL 5.6.51/8.4 与 PostgreSQL。不能将基线结果冒充新增采集版本的验收。

未提供真实通知账号、公网域名、70 台目标机器及 ARM64 实机；未验证这些环境的实际投递、ACME、长期容量或全部 OS 版本。生产分批检查见 [生产验收](PRODUCTION.md)。

---

以下是历史版本的验证记录，其中“未实现”描述只适用于当时版本。

# 第一阶段验证记录

日期：2026-10-28。环境：Windows，最终构建工具链 Go 1.26.10、Node.js 24.16.0。

已完成：

- `go test ./...`：通过；未提供测试 DSN 时数据库测试显式跳过。
- `go vet ./...`：通过。
- PostgreSQL 16.14 独立本地实例：真实数据库集成测试通过。
- MySQL 8.4.13 官方发行包独立本地实例：相同集成测试通过。
- 双数据库验证内容：迁移首次执行/重复执行、登录、Cookie 属性、Origin 拒绝、未登录拒绝、viewer/operator 权限、单次注册凭证重放拒绝、心跳、事件写入、同 ID 重复去重、类型与结果不匹配拒绝、Agent 撤销、事务审计、注销、登录限速。
- Vue TypeScript 类型检查及 Vite 生产构建：通过。
- 浏览器：管理员登录、仪表盘、服务器列表、新增服务器表单提交和保存成功状态已验证。截图 `servers-preview.png` 中数据来自独立测试库。
- Agent 安装/卸载脚本：bash 语法检查通过。
- 升级 Vue 3.5.44、Vite 7.3.7 及 Go 安全依赖后，重新执行双数据库集成测试通过；`npm audit` 报告 0 个漏洞。
- `govulncheck` 报告 0 个可达代码漏洞；另有 3 个导入包漏洞和 1 个依赖模块漏洞未被当前代码调用，仍需持续跟进依赖升级。
- 已在 Windows 交叉编译 Linux amd64 API、Linux amd64/arm64 Agent；离线创建项目脚本已执行并验证生成源码。

未验证范围：

- 当前机器未安装 Docker Engine，未执行 Compose 容器启动。
- 当前机器为 Windows，未在真实 Linux systemd/Nginx 环境运行发布包和 Agent 安装脚本。
- 未绑定真实公网域名、签发证书或在实际 70 台 Linux 上压测。
- 未实现/验收真实 SSH 日志采集、断线队列和通知发送；不得据此认为完整安全监控已上线。

发布前请在 2–3 台测试 Linux 上按 README 完成 HTTPS、Agent 注册/心跳和数据库备份恢复验收，再进入下一阶段采集开发。

## Agent v0.2.0 多平台扩展

- 已加入 Windows SCM 原生服务和 LocalService 运行账号，Ubuntu/Debian systemd、Alpine OpenRC 安装/卸载脚本。
- `go test ./...`、`go vet ./...` 通过。新增 Agent 测试覆盖注册配置首次写入/替换、注册失败保留旧凭证、拒绝未信任 HTTPS 证书、在途心跳请求取消、Windows 服务 Stop/Shutdown/Interrogate 状态处理。
- Windows 安装器 ACL helper 在独立工作区目录实测通过：仅 Administrators/SYSTEM 完全控制、LocalService 读/执行，无普通用户权限。
- PostgreSQL API 集成回归通过，包含返回 Windows/Ubuntu/Debian/Alpine 四套安装命令。数据库结构和 Agent HTTP 协议保持兼容，本轮未修改 MySQL 查询。
- Vue 类型检查及生产构建通过；Windows/Linux、amd64/arm64 四个 Agent 构建成功。Linux 产物检查无 ELF 动态解释器依赖，Windows PE 架构和四个 SHA-256 校验和均验证。
- Windows 安装/卸载脚本通过 PowerShell AST 语法检查；Linux 安装/卸载脚本通过 shell 语法检查。原生 Windows amd64 程序的 `-version` 已执行。
- 本轮**未在宿主机注册真实 Windows 服务**，也未在实际 Ubuntu/Debian/Alpine 系统中启动服务；SCM 的测试为 handler 状态与取消行为测试。仍须用目标机器验证安装、服务启动、停止、重启后恢复和 HTTPS 注册/心跳。Windows ARM64 为交叉编译，未进行设备运行验证。
- 各系统的真实 SSH 日志采集仍属于第二阶段，Windows OpenSSH Event Log、Linux journal/auth.log/secure 的采集器尚未实现。

平台实现参考：[Go Windows service API](https://pkg.go.dev/golang.org/x/sys/windows/svc)、[OpenRC supervise-daemon](https://github.com/OpenRC/openrc/blob/master/supervise-daemon-guide.md)、[Alpine init scripts](https://wiki.alpinelinux.org/wiki/Writing_Init_Scripts)。

## GitHub 云端验证

[首次 main CI](https://github.com/xiuzhenzhi/SSH/actions/runs/40659110222) 已成功：Ubuntu/Windows Go 测试与 vet、Linux race、Windows ACL helper、PostgreSQL/MySQL 容器中的真实数据库集成测试、Vue 构建及依赖检查，以及两个中央平台包与四个 Agent 的构建/校验。产物名为 sshguard-release-assets。

这补充了云端 Linux 构建和数据库容器测试证据；仍不等同于在真实业务主机完成 systemd/OpenRC/Windows SCM 安装及 SSH 日志采集验收。后续运行结果以 Actions 中对应提交为准。
