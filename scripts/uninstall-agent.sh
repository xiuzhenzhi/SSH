#!/bin/sh
set -eu
[ "$(id -u)" -eq 0 ] || { echo 'Run as root.' >&2; exit 1; }
printf '[1/2] 停止 Agent 及版本更新服务…\n'
for path in /opt/SSHGuard /opt/SSHGuard/Agent /opt/SSHGuard/Agent/agent.json /opt/SSHGuard/Agent/sshguard-agent /opt/SSHGuard/Agent/logs /etc/syslog.conf /opt/SSHGuard/Agent/sshguard /opt/SSHGuard/Agent/sshguard-agent-bin /etc/systemd/system/sshguard-agent.service /etc/init.d/sshguard-agent /etc/systemd/system/sshguard-agent-update.timer /etc/systemd/system/sshguard-agent-update.service /etc/init.d/sshguard-agent-update; do
    [ ! -L "$path" ] || { echo "Refusing symlink: $path" >&2; exit 1; }
done
if [ -f /etc/systemd/system/sshguard-agent.service ]; then
    grep -Fxq 'ExecStart=/opt/SSHGuard/Agent/sshguard-agent-bin -config /opt/SSHGuard/Agent/agent.json' /etc/systemd/system/sshguard-agent.service || { echo '同名服务属于其他安装，停止卸载。' >&2; exit 1; }
fi
if [ -f /etc/init.d/sshguard-agent ]; then
    grep -Fxq 'command="/opt/SSHGuard/Agent/sshguard-agent-bin"' /etc/init.d/sshguard-agent || { echo '同名服务属于其他安装，停止卸载。' >&2; exit 1; }
fi
mkdir /run/sshguard-agent-install.lock 2>/dev/null || { echo 'Installation/update active; retry after it finishes.' >&2; exit 1; }
trap 'rmdir /run/sshguard-agent-install.lock' EXIT
if command -v systemctl >/dev/null 2>&1 && [ -f /etc/systemd/system/sshguard-agent-update.timer ]; then
    systemctl disable --now sshguard-agent-update.timer
    systemctl stop sshguard-agent-update.service
    rm -f /etc/systemd/system/sshguard-agent-update.timer /etc/systemd/system/sshguard-agent-update.service
fi
if command -v rc-service >/dev/null 2>&1 && [ -f /etc/init.d/sshguard-agent-update ]; then
    if rc-service sshguard-agent-update status >/dev/null 2>&1; then rc-service sshguard-agent-update stop; fi
    rc-update del sshguard-agent-update default
    rm -f /etc/init.d/sshguard-agent-update
fi
if command -v systemctl >/dev/null 2>&1 && [ -f /etc/systemd/system/sshguard-agent.service ]; then
    systemctl disable --now sshguard-agent
    rm -f /etc/systemd/system/sshguard-agent.service
    systemctl daemon-reload
elif command -v rc-service >/dev/null 2>&1 && [ -f /etc/init.d/sshguard-agent ]; then
    if rc-service sshguard-agent status >/dev/null 2>&1; then rc-service sshguard-agent stop; fi
    rc-update del sshguard-agent default
    rm -f /etc/init.d/sshguard-agent
fi
rm -f /opt/SSHGuard/Agent/sshguard-agent-bin
rm -f /opt/SSHGuard/Agent/sshguard/update-agent.sh /opt/SSHGuard/Agent/sshguard/update-loop.sh /opt/SSHGuard/Agent/sshguard/uninstall-agent.sh
# Remove only the installer-owned syslog destination before deleting its files.
if [ -f /etc/syslog.conf ] && grep -Fxq 'auth,authpriv.* /opt/SSHGuard/Agent/logs/auth.log' /etc/syslog.conf; then
    sed -i '\|^auth,authpriv\.\* /opt/SSHGuard/Agent/logs/auth\.log$|d' /etc/syslog.conf
    if command -v rc-service >/dev/null 2>&1 && rc-service syslog status >/dev/null 2>&1; then rc-service syslog restart; fi
fi
rm -f /opt/SSHGuard/Agent/agent.json
rm -rf /opt/SSHGuard/Agent/sshguard-agent /opt/SSHGuard/Agent/logs
rmdir /opt/SSHGuard/Agent/sshguard 2>/dev/null || true
rmdir /opt/SSHGuard/Agent /opt/SSHGuard 2>/dev/null || true
if command -v systemctl >/dev/null 2>&1; then systemctl daemon-reload; fi
printf '[2/2] Agent 已卸载，本地配置、凭据、队列、采集位置、隔离数据和专用日志已清除。\n请在控制台撤销凭据，或删除服务器及其中央历史记录。系统共享日志不做整库清空。\n'
