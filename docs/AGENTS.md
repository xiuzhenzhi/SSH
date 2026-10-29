# Agent 安装与管理

## 保留身份的一键更新

先更新中央平台，再在服务器列表点击「更新命令」。Linux 使用同一安装器的 `--update`，Windows 使用 `-Update`，不需要新的注册令牌。新程序下载和 SHA256 校验完成后才停止服务；更新会核对原配置 endpoint，服务启动/心跳检查失败恢复旧程序。同版本重复执行不会重新注册。后台 Agent 版本随心跳更新。

凭证已撤销时更新会拒绝，必须由管理员重新生成安装命令。首次安装或重新注册会更换凭证，不能当作更新使用。Linux 凭证文件为 root 所有、服务组只读；Windows 凭证仅 Administrators/SYSTEM 可写，LocalService 只读。SHA256 是完整性检查，信任来源仍是平台 HTTPS。

## 平台触发更新与卸载

新版安装器会安装独立的维护服务，管理员可在服务器列表点击「平台更新」，更新到中央平台提供的 Agent 版本。旧 Agent 先执行一次「更新命令」，之后入口才会显示就绪。维护服务每分钟出站检查一次，不开放入站端口；采集进程仍使用原有低权限账号。

维护服务只运行安装时保存的本地固定更新器，任务只有 ID 和版本号，不接受 shell、脚本、下载地址或其他命令参数。待领取任务 15 分钟过期，已领取任务 10 分钟超时，不自动重复执行；任务绑定目标服务器及当前凭证。请求和执行结果记入操作审计。平台版本在执行前发生变化时拒绝更新，失败后检查日志再重新发起；凭证撤销不能保证中止已开始的本地替换。

- Ubuntu/Debian：`sshguard-agent-update.timer` 每分钟运行一次固定的 root 维护任务；查看 `journalctl -u sshguard-agent-update`。
- Alpine：独立 OpenRC 服务 `sshguard-agent-update`，检查 `rc-service sshguard-agent-update status`。
- Windows：计划任务 `SSHGuardAgentUpdate` 以 SYSTEM 执行；采集服务仍是 LocalService，可在任务计划程序检查结果。

不需要平台更新时，可以在本机禁用上述维护服务或计划任务，采集上报不受影响。后续重新执行安装/更新脚本会重新启用维护服务。维护脚本逻辑升级需要手动执行新版安装器，平台不会远程下载并执行脚本。

点击「卸载命令」，选择系统并在目标机器以管理员身份执行。卸载会移除采集及更新服务，保留配置、队列和历史数据；再在平台点击「撤销」使凭证失效。平台没有远程卸载或通用命令执行功能。

> 安装后分别检查连接状态与采集状态；在线心跳不代表日志源可读。

### 4. 登录与接入服务器

访问 `https://guard.example.com`，用户名 `admin`，密码是你配置的 `ADMIN_PASSWORD`。**仅空用户表时初始化；修改环境变量不会重置现有密码。**

进入「服务器管理」→「添加服务器」→「安装命令」，选择 Windows / Ubuntu / Debian / Alpine 后，在对应目标机器执行。所有选项共享同一条短期凭证，切换选项不会刷新有效期；仅在一台机器执行，不能用同一命令接入多台机器。API 返回 `commands.windows/ubuntu/debian/alpine`，保留 `command` 字段为 Ubuntu 命令兼容旧客户端。

| Agent 系统 | 架构 | 后台运行方式 | 执行安装的位置 |
|---|---|---|---|
| Windows 10/11、Server 2019+ | amd64 / arm64（系统支持相应架构时） | 原生 Windows SCM 服务 `SSHGuardAgent`，自动延迟启动、失败重启 | 管理员 PowerShell 5.1+ |
| Ubuntu | amd64 / arm64 | systemd `sshguard-agent` | 有 sudo 权限的终端 |
| Debian | amd64 / arm64 | systemd `sshguard-agent` | 有 sudo 权限的终端；纯 root 环境可下载脚本后用 sh 执行 |
| Alpine | amd64 / arm64 | OpenRC `sshguard-agent`，supervise-daemon 自动重启 | root 的 sh/ash，不需要 sudo/Bash/glibc |

Windows 系统范围为构建/部署目标，未代表每个 OS 版本均已完成实机验收。Alpine 必须运行 OpenRC；未启用 init 的容器不属于服务安装场景，可自行以前台进程运行 Agent。Linux 发行版版本信息从 `/etc/os-release` 识别，Windows 上报 Windows 与 CPU 架构。

安装前置依赖：

```bash
# Ubuntu / Debian
sudo apt-get update
sudo apt-get install -y curl ca-certificates

# Alpine：在 root shell 中执行，系统须已有 OpenRC
apk add --no-cache curl ca-certificates
```

Windows 使用自带 PowerShell 下载并校验 SHA-256，无需 NSSM、WinSW、Go 或 Docker。脚本的 ExecutionPolicy 参数仅用于当前安装子进程，不修改系统持久策略；受组织策略限制时按组织的签名/分发流程执行脚本。安装不创建防火墙入站规则。

所有 Agent 主动出站访问平台，不需要开放入站端口，不保存 SSH 密码/私钥，也不执行中心下发命令。Linux 二进制使用 `CGO_ENABLED=0`，Ubuntu/Debian 的 glibc 与 Alpine 的 musl 共用静态二进制。所有下载和 API 请求验证 HTTPS 证书，不接受忽略证书验证。

命令包含短期凭证，避免公开粘贴；长期令牌的存放与权限：

- Linux：`/etc/sshguard/agent.json`（0640，root:sshguard）；共享父目录 root:root、0711，只允许穿越，不能列目录或替换条目；API 配置保持 0600，内置 TLS 子目录单独授权 API 用户。v0.2 旧文件权限会在更新时加固。
- Windows：`%ProgramData%\SSHGuard\agent.json`；ACL 只授权 Administrators/SYSTEM 完全控制、LocalService 读取，普通用户无访问权限。程序在 `%ProgramFiles%\SSHGuardAgent`，服务不以管理员账号运行。

注册响应因网络断开丢失时，重新生成安装命令重试；旧令牌会被替换。重新安装期间短暂停止旧服务，注册失败时尝试恢复旧服务。安装凭证单次消耗，成功后再执行旧命令会失败，需要后台生成新命令。

检查服务：

```powershell
# Windows
Get-Service SSHGuardAgent
Get-WinEvent -FilterHashtable @{LogName='Application'; ProviderName='SSHGuardAgent'} -MaxEvents 20
```

```bash
# Ubuntu / Debian
systemctl status sshguard-agent
journalctl -u sshguard-agent -n 50 --no-pager
# Alpine
rc-service sshguard-agent status
grep sshguard-agent /var/log/messages
```

新安装的 Alpine Agent 交给 syslog 管理日志与轮转，需启用本机 syslog 服务；更新会刷新服务定义并保留配置及队列；旧日志文件仍需按本机策略清理。Windows 交由事件日志保留策略管理。卸载脚本：`scripts/uninstall-agent.sh`（root）或 `scripts/uninstall-agent.ps1`（管理员 PowerShell）。卸载保留凭证文件和历史日志，须在后台撤销凭证；系统用户/事件源不自动删除。


## 单台验收

1. 安装完成后服务处于运行状态。
2. 后台出现主机名、操作系统与 Agent 版本，90 秒内显示在线。
3. 重启主机后自动恢复心跳。
4. 撤销凭证后心跳返回 401，不能继续上传事件。
5. 新命令重新注册后旧凭证失效，新凭证恢复心跳。

6. 执行一次成功及失败 SSH 登录，核对事件，再验证通知实际到达。
7. 断开平台网络后生成事件，恢复网络，确认队列清空且事件未重复。

不要克隆已注册的配置或 state.db 到其他机器。

## 采集源与持久化

Linux 优先使用正在运行的 systemd journal（sshd / sshd-session），否则选择 auth.log、secure、messages。Windows 使用 OpenSSH/Operational，需先安装并运行 Windows OpenSSH Server。仅监控 OpenSSH 登录，不采集 Windows RDP、图形登录或通用安全日志。

首次启动从当前日志尾部开始；后续按 journal cursor、Windows EventRecordID 或文件身份+字节偏移恢复。文件改名轮转会先读取旧文件；旧文件已删除、压缩或日志清空时报告采集缺口，不能恢复操作系统已删除的日志。复制截断与写入同时发生仍可能丢失源日志，应优先使用 journal 或改名轮转。

可在 agent.json 增加 log_file 指定可读的 syslog 格式 SSH 日志文件。文件时间支持 RFC3339 / 常见 syslog 时间。不要指定无关日志或可由普通用户写入的文件。

- Linux 状态：/var/lib/sshguard-agent/state.db，0700 专用目录，服务用户可写；安装器加入存在的 adm / systemd-journal 组用于读日志。
- Windows 状态：%ProgramData%\SSHGuard\state\state.db，只给 SSHGuardAgent 服务 SID 修改权限；OpenSSH 通道仅追加该服务的读取权限，不授予通用 Event Log Readers。
- 队列最多 10,000 个事件，满时暂停采集，不推进读取位置。服务端明确拒绝的记录单独隔离（上限 1,000），隔离满时保留剩余队列。磁盘满或状态文件不可写时会报告异常。
- 更新保留队列和读取位置；卸载保留状态，重新注册为另一台服务器前须人工归档旧状态，不能让未确认事件归属错误服务器。

默认每 2 秒检查日志和待传事件，30 秒心跳。WebSocket 重连采用退避与随机延迟；不可用时走 HTTPS。停机超过操作系统日志保留时间或积压超过 30 天可能产生不可恢复缺口，请监控队列深度与日志空间。
