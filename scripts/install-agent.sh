#!/bin/sh
# POSIX sh: Ubuntu/Debian (systemd) and Alpine (OpenRC). No bash/glibc required.
set -eu
umask 077
[ "$(id -u)" -eq 0 ] || { echo 'Run as root (sudo sh on Ubuntu/Debian; root shell on Alpine).' >&2; exit 1; }
[ "$#" -eq 2 ] || { echo 'Usage: install-agent.sh HTTPS_ORIGIN ONE_TIME_TOKEN' >&2; exit 1; }
endpoint=${1%/}
enrollment=$2
case "$endpoint" in https://*) ;; *) echo 'HTTPS required.' >&2; exit 1 ;; esac
printf '%s' "$enrollment" | grep -Eq '^[a-f0-9]{64}$' || { echo 'Invalid enrollment token.' >&2; exit 1; }
if command -v systemctl >/dev/null 2>&1 && [ -d /run/systemd/system ]; then
    init=systemd
elif command -v rc-service >/dev/null 2>&1 && command -v supervise-daemon >/dev/null 2>&1; then
    init=openrc
else
    echo 'A running systemd or OpenRC installation is required.' >&2; exit 1
fi
command -v curl >/dev/null 2>&1 || { echo 'Install curl and ca-certificates first (apt or apk).' >&2; exit 1; }
command -v sha256sum >/dev/null 2>&1
case "$(uname -m)" in x86_64) arch=amd64 ;; aarch64|arm64) arch=arm64 ;; *) echo 'Only amd64 / arm64 supported.' >&2; exit 1 ;; esac
tmp=$(mktemp -d)
trap 'rm -rf -- "$tmp"' EXIT
curl --proto '=https' --tlsv1.2 -fsS "$endpoint/downloads/sshguard-agent-linux-$arch" -o "$tmp/agent"
curl --proto '=https' --tlsv1.2 -fsS "$endpoint/downloads/sshguard-agent-linux-$arch.sha256" -o "$tmp/hash"
expected=$(awk '{print $1}' "$tmp/hash")
printf '%s' "$expected" | grep -Eq '^[a-f0-9]{64}$' || exit 1
printf '%s  %s\n' "$expected" "$tmp/agent" | sha256sum -c -
if ! id sshguard >/dev/null 2>&1; then
    if command -v useradd >/dev/null 2>&1; then
        useradd --system --user-group --no-create-home --shell /usr/sbin/nologin sshguard
    else
        addgroup -S sshguard
        adduser -S -D -H -h /nonexistent -s /sbin/nologin -G sshguard sshguard
    fi
fi
mkdir -p /etc/sshguard /usr/local/bin
# The daemon may read its credential, but cannot replace directory entries.
chown root:sshguard /etc/sshguard
chmod 0750 /etc/sshguard
was_active=false
if [ "$init" = systemd ]; then
    if systemctl is-active --quiet sshguard-agent; then was_active=true; systemctl stop sshguard-agent; fi
else
    if rc-service sshguard-agent status >/dev/null 2>&1; then was_active=true; rc-service sshguard-agent stop; fi
fi
chmod 0700 "$tmp/agent"
if ! printf '%s' "$enrollment" | "$tmp/agent" -enroll -endpoint "$endpoint" -config /etc/sshguard/agent.json; then
    if "$was_active"; then
        if [ "$init" = systemd ]; then systemctl start sshguard-agent; else rc-service sshguard-agent start; fi
    fi
    echo 'Enrollment failed; generate a new command and retry.' >&2; exit 1
fi
unset enrollment
chown sshguard:sshguard /etc/sshguard/agent.json
chmod 0600 /etc/sshguard/agent.json
cp "$tmp/agent" /usr/local/bin/sshguard-agent
chown root:root /usr/local/bin/sshguard-agent
chmod 0755 /usr/local/bin/sshguard-agent
if [ "$init" = systemd ]; then
    cat > /etc/systemd/system/sshguard-agent.service <<'UNIT'
[Unit]
Description=SSHGuard Agent (phase 1 heartbeat only)
After=network-online.target
Wants=network-online.target
[Service]
User=sshguard
Group=sshguard
ExecStart=/usr/local/bin/sshguard-agent -config /etc/sshguard/agent.json
Restart=on-failure
RestartSec=10
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=true
PrivateTmp=true
UMask=0077
[Install]
WantedBy=multi-user.target
UNIT
    systemctl daemon-reload
    systemctl enable --now sshguard-agent
else
    cat > /etc/init.d/sshguard-agent <<'OPENRC'
#!/sbin/openrc-run
description="SSHGuard Agent (phase 1 heartbeat only)"
supervisor="supervise-daemon"
command="/usr/local/bin/sshguard-agent"
command_args="-config /etc/sshguard/agent.json"
command_user="sshguard:sshguard"
respawn_delay=10
respawn_max=0
output_log="/var/log/sshguard/agent.log"
error_log="/var/log/sshguard/agent.log"
depend() {
    need net
    after firewall
}
start_pre() {
    checkpath --directory --owner sshguard:sshguard --mode 0750 /var/log/sshguard
    checkpath --file --owner sshguard:sshguard --mode 0640 /var/log/sshguard/agent.log
}
OPENRC
    chmod 0755 /etc/init.d/sshguard-agent
    rc-update add sshguard-agent default
    rc-service sshguard-agent start
fi
echo "Installed for $init: registration + heartbeat only; SSH collection is not implemented."
