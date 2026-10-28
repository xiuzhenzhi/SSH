#!/bin/sh
set -eu
[ "$(id -u)" -eq 0 ] || { echo 'Run as root.' >&2; exit 1; }
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
echo 'Configuration and logs retained. Revoke the credential in the dashboard.'
