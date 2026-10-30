# Agent 安装与管理

## 安装

1. 在「服务器管理」添加服务器。
2. 打开「安装命令」，选择 Linux 或 Windows 及上报间隔。Linux 适用于 Ubuntu / Debian / Alpine，会自动识别系统和架构；root 终端直接运行，其他终端使用 sudo。
3. 在目标机器执行命令，确认控制台显示在线及采集正常。

每台服务器使用自己的安装命令，不要复用已注册的配置。

重新执行安装命令注册成功后，Agent 会按新凭证重新开始采集，自动清空旧的本地待传队列、采集位置和隔离数据，不再要求手动删除 `state.db`。检测到状态属于其他主控或服务器，或旧数据没有身份标记时也会重置。此操作不归档旧数据；如需保留未上传事件，请先等队列清空。正常重启和「更新命令 / 平台更新」继续保留本地状态。

修改上报间隔后，安装命令会自动更新，完成后直接复制即可。命令更新期间暂不可复制；失败时点击「重试」。间隔范围为 1–30 秒，只在安装时设置。

| 系统 | 架构 | 执行位置 | 服务 |
|---|---|---|---|
| Windows 10/11、Server 2019+ | amd64 / arm64 | 管理员 PowerShell 5.1+ | `SSHGuardAgent` |
| Ubuntu / Debian | amd64 / arm64 | 有 sudo 权限的终端 | `sshguard-agent` |
| Alpine | amd64 / arm64 | root 终端，需 OpenRC | `sshguard-agent` |

缺少下载依赖时执行：

```sh
# Ubuntu / Debian
sudo apt-get update
sudo apt-get install -y curl ca-certificates
# Alpine
apk add --no-cache curl ca-certificates
```

## 更新

先 [更新主控](QUICKSTART.md#更新中央平台)，再在服务器列表选择：

- **更新命令**：复制到目标机器执行，Linux 参数为 `--update`，Windows 为 `-Update`。
- **平台更新**：由维护服务在目标机器执行，每分钟检查一次任务。

更新保留身份、上报间隔和队列。失败时先查看日志，再重新发起。

Windows 安装及自动更新遵循系统 PowerShell 执行策略，不自动绕过或修改策略；受企业策略限制时，请由管理员允许运行受信任的本地脚本。更新会迁移本安装创建的旧维护任务，其他安装的同名任务仍会被拒绝。

| 系统 | 更新任务与日志 |
|---|---|
| Ubuntu / Debian | `journalctl -u sshguard-agent-update` |
| Alpine | `rc-service sshguard-agent-update status` |
| Windows | 任务计划程序 → `SSHGuardAgentUpdate` |

## 检查服务

```sh
# Ubuntu / Debian
systemctl status sshguard-agent
journalctl -u sshguard-agent -n 50 --no-pager
# Alpine
rc-service sshguard-agent status
tail -n 50 /var/log/sshguard/agent.log
```

```powershell
Get-Service SSHGuardAgent
Get-WinEvent -FilterHashtable @{LogName='Application';ProviderName='SSHGuardAgent'} -MaxEvents 20
```

## 卸载

在控制台复制「卸载命令」，或使用本地脚本：

```sh
# Ubuntu / Debian；Alpine 以 root 执行，去掉 sudo
sudo sh /usr/local/lib/sshguard/uninstall-agent.sh
```

```powershell
& "$env:ProgramFiles\SSHGuardAgent\uninstall-agent.ps1"
```

卸载会永久清除本机 Agent 配置、凭据、待传队列、采集位置、隔离数据和专用日志，并移除更新任务及程序文件；Windows 同时删除 Agent 事件源。系统共享 journal、Windows Application 和 OpenSSH 日志不会整库清空。中央平台数据需在控制台「删除服务器」中清除；只需停用时撤销凭据。

## 删除服务器

管理员在「服务器管理」对应行的「更多维护操作（⋯）」中选择「删除服务器」，核对提示后点击「确认删除」。在线、待接入和已撤销的服务器均可删除。

删除会永久清除该服务器的 SSH 事件、告警及通知投递记录，并使安装和 Agent 凭证失效；操作审计保留。目标机器上的 Agent 需另行卸载，可在删除前复制「卸载命令」。若需保留历史数据，只使用「撤销凭证」。

## 采集与状态文件

- Linux 优先读取 journal，其次读取 SSH 日志文件；可在 `agent.json` 中用 `log_file` 指定日志。
- Windows 读取 `OpenSSH/Operational`，不包含 RDP 登录。
- 首次从日志末尾开始采集，重启后继续读取；离线事件暂存本地，队列上限 10,000 条。
- Linux 配置：`/etc/sshguard/agent.json`；状态：`/var/lib/sshguard-agent/`。
- Windows 配置：`%ProgramData%\SSHGuard\agent.json`；状态：`%ProgramData%\SSHGuard\state\`。

### 资源占用

Linux 和 Windows 使用相同的轻量队列与地址缓存。空闲时采集轮询逐步放缓，最长为安装上报间隔与 10 秒中的较小值；默认安装间隔 5 秒，空闲时最多每 5 秒检查一次。发现日志活动后恢复较快轮询，上报间隔保持安装值。

Linux 的 journal 读取进程用完即退出，不额外常驻；文件采集复用读取位置并减少文件状态查询。队列满时暂停采集，腾出空间后继续。网卡地址每分钟刷新；公网地址按 IPv4 / IPv6 分别缓存，探测失败逐步延长重试间隔，最长 15 分钟。

### HTTP 地址迁移到 HTTPS

1. 等待待传队列清空，备份配置与状态目录。
2. 停止采集服务和更新任务。
3. 通过新地址打开控制台，在原服务器记录生成安装命令并执行；注册成功后自动清空旧本地状态。
4. 确认重新上线，并验证一次真实 SSH 登录。

## 上报设置与 IP 地址

上报间隔在安装时选择，范围 1–30 秒，默认 5 秒；更新时沿用原值。

服务器列表只显示公网 IPv4 / IPv6，NAT 环境显示探测到的公网出口。没有结果时显示「等待识别公网 IP」。国家与旗帜设置见 [地理位置](GEOIP.md)。

### Alpine 采集异常修复

重新执行新版安装命令，可修复标准 BusyBox syslog 的认证日志权限。使用自定义 syslog 或 `log_file` 时，确认 Agent 能读取相应文件。

## 服务器自动命名

添加或编辑服务器时名称可留空，主控会生成 `服务器-xxxxxxxx` 格式的名称。填写的名称会去除首尾空白，最长 128 个字符。`POST /api/v1/servers` 和 `PATCH /api/v1/servers/:id` 的返回数据包含 `id` 和最终 `name`。
