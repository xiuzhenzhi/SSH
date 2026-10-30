# Agent 安装与管理

## 保留身份的一键更新

先更新中央平台，再在服务器列表点击「更新命令」。Linux 使用同一安装器的 `--update`，Windows 使用 `-Update`，不需要新的注册令牌。新程序下载和 SHA256 校验完成后才停止服务；更新会核对原配置 endpoint，服务启动/心跳检查失败恢复旧程序。同版本重复执行不会重新注册。后台 Agent 版本随心跳更新。

凭证已撤销时更新会拒绝，必须由管理员重新生成安装命令。首次安装或重新注册会更换凭证，不能当作更新使用。Linux 凭证文件为 root 所有、服务组只读；Windows 凭证仅 Administrators/SYSTEM 可写，LocalService 只读。SHA256 是完整性检查，HTTPS 部署的信任来源仍是平台证书；HTTP 直连的凭证、下载和上报均未加密，应使用可信网络。

## 平台触发更新与卸载

安装器会安装独立的维护服务，管理员可在服务器列表点击「平台更新」，更新到主控指定版本。维护服务每分钟出站检查一次，不开放入站端口；采集进程仍使用原有低权限账号。

维护服务只运行安装时保存的本地固定更新器，任务只有 ID 和版本号，不接受 shell、脚本、下载地址或其他命令参数。待领取任务 15 分钟过期，已领取任务 10 分钟超时，不自动重复执行；任务绑定目标服务器及当前凭证。请求和执行结果记入操作审计。平台版本在执行前发生变化时拒绝更新，失败后检查日志再重新发起；凭证撤销不能保证中止已开始的本地替换。

- Ubuntu/Debian：`sshguard-agent-update.timer` 每分钟运行一次固定的 root 维护任务；查看 `journalctl -u sshguard-agent-update`。
- Alpine：独立 OpenRC 服务 `sshguard-agent-update`，检查 `rc-service sshguard-agent-update status`。
- Windows：计划任务 `SSHGuardAgentUpdate` 以 SYSTEM 执行；采集服务仍是 LocalService，可在任务计划程序检查结果。

不需要平台更新时，可以在本机禁用上述维护服务或计划任务，采集上报不受影响。后续重新执行安装/更新脚本会重新启用维护服务。维护脚本逻辑升级需要手动执行新版安装器，平台不会远程下载并执行脚本。

点击「卸载命令」，选择系统并在目标机器以管理员身份执行。卸载会移除采集及更新服务，保留配置、队列和历史数据；再在平台点击「撤销」使凭证失效。平台没有远程卸载或通用命令执行功能。

> 安装后分别检查连接状态与采集状态；在线心跳不代表日志源可读。

### 4. 登录与接入服务器

通过主控 IP 和端口（如 `http://192.0.2.10:8080`）或已配置的 HTTPS 地址打开控制台。网页初始化的安装使用自己创建的管理员；配置文件安装使用 `ADMIN_USERNAME` / `ADMIN_PASSWORD`。**仅空用户表时初始化；修改环境变量不会重置现有密码。**

进入「服务器管理」→「添加服务器」→「安装命令」，选择 Windows / Ubuntu / Debian / Alpine 后，在对应目标机器执行。所有选项共享同一条短期凭证，切换选项不会刷新有效期；仅在一台机器执行，不能用同一命令接入多台机器。API 返回 `commands.windows/ubuntu/debian/alpine`，`command` 字段为 Ubuntu 命令。

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

Windows 使用系统自带 curl.exe 下载、PowerShell 校验 SHA-256，无需 NSSM、WinSW、Go 或 Docker。控制台生成的 Windows 命令不主动指定 ExecutionPolicy，也不修改系统策略；受组织策略限制时按组织批准的签名/分发流程执行脚本。固定维护任务保留既有的本地执行机制。安装不创建防火墙入站规则。

控制台生成的安装脚本和 Agent 二进制直接下载自本项目 GitHub Release，固定到主控版本对应的 `v版本号`，不使用 latest 或主控的下载目录。下载通过 HTTPS，限制重定向、文件大小与超时；二进制的 SHA-256 和版本号均通过后才替换服务。目标机器须能访问 GitHub 及其 Release 资源域名，下载失败不切换运行中的程序。主控 IP / 域名仅用于注册、上报与领取固定版本更新任务。

更新保留已安装的上报间隔、身份凭证与队列。

所有 Agent 主动出站访问平台，不需要开放入站端口，不保存 SSH 密码/私钥，也不执行中心下发命令。Linux 二进制使用 `CGO_ENABLED=0`，Ubuntu/Debian 的 glibc 与 Alpine 的 musl 共用静态二进制。HTTPS 下载和 API 请求严格校验证书，不接受忽略证书验证。原生默认安装也支持无域名 HTTP 直连，控制台生成的命令会显式附加 `--allow-insecure-http` / `-AllowInsecureHttp`，并在本地 Agent 配置保存 `allow_insecure_http=true`。未经显式授权的 HTTP 仍被拒绝；更新沿用原地址和授权，不自动切换协议。

命令包含短期凭证，避免公开粘贴；长期令牌的存放与权限：

- Linux：`/etc/sshguard/agent.json`（0640，root:sshguard）；共享父目录 root:root、0711，只允许穿越，不能列目录或替换条目；API 配置保持 0600，内置 TLS 子目录单独授权 API 用户。
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


## 简单安装与离线卸载

在控制台添加服务器，复制对应系统的一条安装命令到目标机器执行。Linux 自动识别 amd64/arm64 和 systemd/OpenRC，安装所需依赖、注册服务并设置开机自启；Windows 使用管理员 PowerShell，服务以 LocalService 运行。更新沿用服务器身份和配置，支持平台管理员触发固定版本更新；不支持远程 Shell、文件操作或其他命令。

新安装/更新会缓存卸载脚本，平台不可访问时也能执行：

```sh
# Ubuntu / Debian；Alpine 使用 root 执行，去掉 sudo
sudo sh /usr/local/lib/sshguard/uninstall-agent.sh
```

```powershell
# 管理员 PowerShell
& "$env:ProgramFiles\SSHGuardAgent\uninstall-agent.ps1"
```

卸载停止并删除采集服务、受限更新服务/计划任务及程序，保留配置、队列和历史日志，不影响中央平台。也可使用控制台生成的卸载命令，或复制仓库中的对应卸载脚本执行。之后在控制台撤销凭据。安装与卸载体验参考 [Komari Agent](https://github.com/komari-monitor/komari-agent)，本项目仍保持专用 SSH 采集工具的权限范围。

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

Linux 优先使用正在运行的 systemd journal（sshd / sshd-session），否则依次选择专用 sshguard/auth.log、auth.log、secure、messages 中可读的普通文件。Windows 使用 OpenSSH/Operational，需先安装并运行 Windows OpenSSH Server。仅监控 OpenSSH 登录，不采集 Windows RDP、图形登录或通用安全日志。

首次启动从当前日志尾部开始；后续按 journal cursor、Windows EventRecordID 或文件身份+字节偏移恢复。文件改名轮转会先读取旧文件；旧文件已删除、压缩或日志清空时报告采集缺口，不能恢复操作系统已删除的日志。复制截断与写入同时发生仍可能丢失源日志，应优先使用 journal 或改名轮转。

可在 agent.json 增加 log_file 指定可读的 syslog 格式 SSH 日志文件。文件时间支持 RFC3339 / 常见 syslog 时间。不要指定无关日志或可由普通用户写入的文件。

- Linux 状态：/var/lib/sshguard-agent/state.db，0700 专用目录，服务用户可写；安装器加入存在的 adm / systemd-journal 组用于读日志。
- Windows 状态：%ProgramData%\SSHGuard\state\state.db，只给 SSHGuardAgent 服务 SID 修改权限；OpenSSH 通道仅追加该服务的读取权限，不授予通用 Event Log Readers。
- 队列最多 10,000 个事件，满时暂停采集，不推进读取位置。服务端明确拒绝的记录单独隔离（上限 1,000），隔离满时保留剩余队列。磁盘满或状态文件不可写时会报告异常。
- 更新保留队列和读取位置；卸载保留状态，重新注册为另一台服务器前须人工归档旧状态，不能让未确认事件归属错误服务器。

### HTTP 地址迁移到 HTTPS

队列身份绑定 `endpoint + server_id`。即使沿用同一条服务器记录，更改协议、IP、域名或端口后，Agent 也会拒绝直接使用原状态库。先让待传队列归零并备份，停止采集服务与更新定时器 / 计划任务，再把整个状态目录重命名归档（Linux：`/var/lib/sshguard-agent`；Windows：`%ProgramData%\SSHGuard\state`；使用新的归档名称，不能覆盖旧备份）。Alpine 需同时停止对应 OpenRC 服务。

然后通过新 HTTPS 地址打开主控，在原服务器记录生成新安装命令并执行。安装器创建新的状态目录；检查上线和采集健康，验证真实登录。新状态从当前日志尾部开始，切换窗口可能有采集间隙，原日志和归档需保留。未上传队列应先完成恢复处理，不能靠删除旧状态绕过错误；不要把旧 state.db 复制到新身份目录。此操作不会由普通更新命令自动完成，详细步骤见 [反代与 HTTPS](REVERSE-PROXY.md)。

默认每 2 秒检查日志和待传事件，30 秒心跳。WebSocket 重连采用退避与随机延迟；不可用时走 HTTPS。停机超过操作系统日志保留时间或积压超过 30 天可能产生不可恢复缺口，请监控队列深度与日志空间。

### 内存与资源边界

事件存储在本地持久队列，上限 10,000 条，隔离区上限 1,000 条；每次最多读取 100 条上报。文件 / journal 采集每轮最多处理 500 行，单行最大 64 KiB；Windows 事件输出最多 4 MiB、每轮最多 100 条。journal 诊断输出最多保留 4 KiB，轮转目录每批扫描 128 个条目。

WebSocket 失败与退出会关闭连接，HTTP 响应体均关闭；采集轮询显式停止 ticker、取消上下文并等待采集 goroutine，之后关闭状态库。资源回收测试覆盖重复断线和启动停止，不能把 Go 的堆保留、操作系统工作集或 bbolt 的文件映射高水位直接当成泄漏。持续增长应同时核对队列深度、GC 后存活堆、goroutine / 文件句柄和日志源异常。

## 上报设置与 IP 地址

上报间隔仅能在安装窗口选择，`report_interval_seconds` 为 1–30，默认 5 秒；生成命令的单次凭证绑定选定值，注册成功时写入本地配置。它控制心跳及正常事件批量上传间隔，采集源仍以至多 2 秒的间隔检查（设为 1 秒时同步加快）。失败上传使用退避，最多每轮处理 10 批，不承诺繁忙或离线时仍固定按时到达。

编辑服务器与更新窗口不提供此设置，API 拒绝运行时修改。Agent 只从注册响应固化安装间隔，心跳确认不下发配置或其他命令，更新保留原配置。Agent 运行期保留 SSH 采集上报和控制台请求的固定版本更新，安装 / 卸载由本机维护脚本负责。

服务器列表和详情只显示公网 IPv4 / IPv6，过滤 Docker 网桥、LXC、私网、回环、链路本地和保留地址。Agent 枚举网卡并在后台通过 HTTPS 分别探测 IPv4 / IPv6 公网出口（ipify，失败时尝试 icanhazip），最多上报 16 个地址；探测不携带平台令牌，不使用环境代理，不阻塞日志上传。双栈结果缓存 15 分钟，缺少地址族时每分钟重试，旧的成功结果最多保留 1 小时。新启动通常在下一次心跳展示公网结果。

主控还会使用认证心跳连接的公网来源补充缺少的地址族；转发头仅接受 `TRUSTED_PROXY_CIDRS` 指定的反代。NAT 显示的是公网出口，运营商的入站端口映射地址可能与出口不同，无法由容器网卡推导。没有可靠公网结果时显示「等待识别公网 IP」。名称前旗帜和 GeoIP 数据源配置见 [地理位置](GEOIP.md)。主控与 Agent 都需更新到包含此功能的版本。

### Alpine 采集异常修复

Alpine 标准 BusyBox syslog 服务使用 wheel 组和 027 掩码。安装器在 `/etc/syslog.conf` 中追加认证设施的专用目的地 `/var/log/sshguard/auth.log`；目录 root:sshguard、2750，日志 root:sshguard、0640，轮转后新文件继承组。保留原 syslog 规则，Agent 不加入管理员 wheel 组。其他日志设施不会进入专用文件。

重新执行安装器可检查并修复专用日志权限。自定义 `SYSLOGD_OPTS` 的文件 / 网络 / 缓冲选项会保留并提示人工配置；syslog-ng 等其他日志服务需自行为 sshguard 提供可读的认证日志。显式 `log_file` 优先，不会被专用路径覆盖。

无新增文件内容时不写入状态库；事件、游标仍在同一 fsync 事务中持久化。心跳持续失败时诊断最多每分钟一条，恢复时记录一次。连接、定时器、日志子进程及状态库在取消时释放，不以丢弃队列换取低写入。

浏览器的实时刷新提示按每个会话 5 秒合并，总览共享 5 秒统计缓存；不会为每个 Agent 心跳重复查询总览。页面显示延迟与网络和缓存有关，上报设置不代表浏览器逐秒刷新。
