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
