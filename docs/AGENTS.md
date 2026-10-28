# Agent 安装与管理

> 当前仅注册与心跳，尚不采集 SSH 日志。

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

- Linux：`/etc/sshguard/agent.json`（0600，sshguard 所有）；父目录 root:sshguard、0750，服务账号不能替换目录条目。
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
tail -n 50 /var/log/sshguard/agent.log
```

Alpine 文件日志需结合本机 logrotate 管理保留期；Windows 交由事件日志保留策略管理。卸载脚本：`scripts/uninstall-agent.sh`（root）或 `scripts/uninstall-agent.ps1`（管理员 PowerShell）。卸载保留凭证文件和历史日志，须在后台撤销凭证；系统用户/事件源不自动删除。


## 单台验收

1. 安装完成后服务处于运行状态。
2. 后台出现主机名、操作系统与 Agent 版本，90 秒内显示在线。
3. 重启主机后自动恢复心跳。
4. 撤销凭证后心跳返回 401，不能继续上传事件。
5. 新命令重新注册后旧凭证失效，新凭证恢复心跳。

在线不代表采集与告警已经启用。不要克隆已注册的配置到其他机器。
