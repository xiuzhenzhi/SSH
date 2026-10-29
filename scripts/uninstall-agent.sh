#!/bin/sh
set -eu
[ "$(id -u)" -eq 0 ] || { echo 'Run as root.' >&2; exit 1; }
for path in /usr/local/lib/sshguard /usr/local/bin/sshguard-agent /etc/systemd/system/sshguard-agent.service /etc/init.d/sshguard-agent; do
    [ ! -L "$path" ] || { echo "Refusing symlink: $path" >&2; exit 1; }
done
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
rm -f /usr/local/bin/sshguard-agent
rm -f /usr/local/lib/sshguard/update-agent.sh /usr/local/lib/sshguard/update-loop.sh
echo 'Configuration and logs retained. Revoke the credential in the dashboard.'
