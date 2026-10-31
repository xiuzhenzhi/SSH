# 卸载 Agent

在目标机器执行对应系统的命令，无需下载或调用卸载脚本。以下使用默认安装目录；执行前等待正在进行的安装或更新结束。

卸载会永久删除本机 Agent 配置、凭据、待传队列、采集位置和专用日志，需要保留时先备份。

## Ubuntu / Debian（systemd）

使用有 sudo 权限的终端；root 用户可去掉 `sudo`。

```sh
sudo systemctl disable --now sshguard-agent-update.timer &&
sudo systemctl stop sshguard-agent-update.service &&
sudo systemctl disable --now sshguard-agent &&
sudo rm -f /etc/systemd/system/sshguard-agent.service \
  /etc/systemd/system/sshguard-agent-update.service \
  /etc/systemd/system/sshguard-agent-update.timer &&
sudo systemctl daemon-reload &&
sudo rm -rf /opt/SSHGuard/Agent
```

## Alpine（OpenRC）

以 root 执行，同时移除安装器添加的专用 syslog 规则。

```sh
rc-service sshguard-agent-update stop &&
rc-update del sshguard-agent-update default &&
rc-service sshguard-agent stop &&
rc-update del sshguard-agent default &&
rm -f /etc/init.d/sshguard-agent /etc/init.d/sshguard-agent-update &&
if [ -f /etc/syslog.conf ]; then
  sed -i '\|^auth,authpriv\.\* /opt/SSHGuard/Agent/logs/auth\.log$|d' /etc/syslog.conf &&
  rc-service syslog restart
fi &&
rm -rf /opt/SSHGuard/Agent
```

## Windows

以管理员身份打开 PowerShell，停止更新任务和服务后删除 Agent 文件。

```powershell
$ErrorActionPreference = 'Stop'
if (Get-ScheduledTask -TaskName SSHGuardAgentUpdate -ErrorAction SilentlyContinue) {
    Disable-ScheduledTask -TaskName SSHGuardAgentUpdate | Out-Null
    Stop-ScheduledTask -TaskName SSHGuardAgentUpdate
    Unregister-ScheduledTask -TaskName SSHGuardAgentUpdate -Confirm:$false
}
$service = Get-Service -Name SSHGuardAgent -ErrorAction SilentlyContinue
if ($service) {
    Stop-Service -Name SSHGuardAgent
    $service.WaitForStatus('Stopped', [TimeSpan]::FromSeconds(30))
    $service.Dispose()
    sc.exe delete SSHGuardAgent
    if ($LASTEXITCODE -ne 0) { throw '服务删除失败，请处理错误后重试。' }
}
if ([Diagnostics.EventLog]::SourceExists('SSHGuardAgent')) {
    Remove-EventLog -Source SSHGuardAgent
}
Remove-Item -LiteralPath "$env:ProgramData\SSHGuard\Agent" -Recurse -Force
```

主控及系统共享日志保留。完成后在控制台撤销凭据；如需清除平台历史数据，使用 [删除服务器](AGENTS.md#删除服务器)。
