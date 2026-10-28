#!/bin/sh
# Ubuntu/Debian: systemd. Alpine: OpenRC. No bash, Python, Node or container.
set -eu
umask 077
die() { echo "$*" >&2; exit 1; }
[ "$(id -u)" -eq 0 ] || die 'Run as root (sudo sh on Ubuntu/Debian).'
[ "$#" -eq 2 ] || die 'Usage: install-agent.sh HTTPS_ORIGIN ONE_TIME_TOKEN|--update'
endpoint=${1%/}
mode=install
enrollment=$2
printf '%s' "$endpoint" | grep -Eq '^https://[A-Za-z0-9.-]+(:[0-9]{1,5})?$' || die 'Expected an HTTPS DNS origin.'
if [ "$enrollment" = --update ]; then mode=update
else printf '%s' "$enrollment" | grep -Eq '^[a-f0-9]{64}$' || die 'Invalid enrollment token.'; fi
if command -v systemctl >/dev/null 2>&1 && [ -d /run/systemd/system ]; then init=systemd
elif command -v rc-service >/dev/null 2>&1 && command -v supervise-daemon >/dev/null 2>&1; then init=openrc
else die 'A running systemd or OpenRC installation is required.'; fi
command -v curl >/dev/null 2>&1 || die 'Install curl and ca-certificates first (apt or apk).'
command -v sha256sum >/dev/null 2>&1
case "$(uname -m)" in x86_64) arch=amd64 ;; aarch64|arm64) arch=arm64 ;; *) die 'Only amd64 / arm64 supported.' ;; esac
for path in /var/lib/sshguard-agent /etc/sshguard /etc/sshguard/agent.json /usr/local/bin/sshguard-agent /etc/systemd/system/sshguard-agent.service /etc/init.d/sshguard-agent; do
    [ ! -L "$path" ] || die "Refusing symlink: $path"
done
config=/etc/sshguard/agent.json
exe=/usr/local/bin/sshguard-agent
if [ "$mode" = update ]; then
    [ -f "$config" ] && [ -f "$exe" ] || die 'Agent not installed; use a new installation command.'
fi
mkdir /run/sshguard-agent-install.lock 2>/dev/null || die 'Another installation is active (or a stale /run/sshguard-agent-install.lock exists).'
tmp=''
rollback=false
was_active=false
service_stop() { if [ "$init" = systemd ]; then systemctl stop sshguard-agent; else rc-service sshguard-agent stop; fi; }
service_start() { if [ "$init" = systemd ]; then systemctl start sshguard-agent; else rc-service sshguard-agent start; fi; }
service_active() { if [ "$init" = systemd ]; then systemctl is-active --quiet sshguard-agent; else rc-service sshguard-agent status >/dev/null 2>&1; fi; }
write_service() {
if [ "$init" = systemd ]; then
    cat > /etc/systemd/system/sshguard-agent.service <<'UNIT'
[Unit]
Description=SSHGuard Agent
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
ReadWritePaths=/var/lib/sshguard-agent
UMask=0077
[Install]
WantedBy=multi-user.target
UNIT
    systemctl daemon-reload
    systemctl enable sshguard-agent
else
    cat > /etc/init.d/sshguard-agent <<'OPENRC'
#!/sbin/openrc-run
description="SSHGuard Agent"
supervisor="supervise-daemon"
command="/usr/local/bin/sshguard-agent"
command_args="-config /etc/sshguard/agent.json"
command_user="sshguard:sshguard"
respawn_delay=10
respawn_max=0
output_logger="logger -t sshguard-agent"
error_logger="logger -t sshguard-agent"
depend() { need net; after firewall; }
OPENRC
    chmod 0755 /etc/init.d/sshguard-agent
    rc-update add sshguard-agent default

fi
}
prepare_collection() {
    mkdir -p /var/lib/sshguard-agent
    chown sshguard:sshguard /var/lib/sshguard-agent
    chmod 0700 /var/lib/sshguard-agent
    for group in adm systemd-journal; do
        if grep -q "^$group:" /etc/group; then
            if command -v usermod >/dev/null 2>&1; then usermod -a -G "$group" sshguard
            else addgroup sshguard "$group"; fi
        fi
    done
}
cleanup() {
    status=$?
    trap - EXIT HUP INT TERM
    if "$rollback"; then
        service_stop || true
        if [ -f "$tmp/previous" ]; then mv -f "$tmp/previous" "$exe"; fi
        if [ -f "$tmp/previous-service" ]; then
            cp -p "$tmp/previous-service" "$service_file"
            if [ "$init" = systemd ]; then systemctl daemon-reload; fi
        fi
        if "$was_active"; then service_start || true; fi
        echo 'Update failed; previous binary restored. Configuration preserved.' >&2
    fi
    [ -z "$tmp" ] || rm -rf -- "$tmp"
    rmdir /run/sshguard-agent-install.lock
    exit "$status"
}
trap cleanup EXIT
trap 'exit 1' HUP INT TERM
mkdir -p /usr/local/bin
tmp=$(mktemp -d /usr/local/bin/.sshguard-install.XXXXXX)
curl --proto '=https' --tlsv1.2 --connect-timeout 15 --max-time 180 --retry 2 -fsS "$endpoint/downloads/sshguard-agent-linux-$arch" -o "$tmp/agent"
curl --proto '=https' --tlsv1.2 --connect-timeout 15 --max-time 30 -fsS "$endpoint/downloads/sshguard-agent-linux-$arch.sha256" -o "$tmp/hash"
expected=$(awk '{print $1}' "$tmp/hash")
printf '%s' "$expected" | grep -Eq '^[a-f0-9]{64}$' || die 'Invalid checksum.'
printf '%s  %s\n' "$expected" "$tmp/agent" | sha256sum -c -
chmod 0755 "$tmp/agent"
"$tmp/agent" -version
if [ "$mode" = update ]; then
    chown root:sshguard "$config"
    chmod 0640 "$config"
    "$tmp/agent" -check -endpoint "$endpoint" -config "$config"
    if cmp -s "$tmp/agent" "$exe"; then
        if ! service_active; then service_start; sleep 2; service_active; fi
        echo 'Already up to date; credentials verified and service running.'; exit 0
    fi
    prepare_collection
    if [ "$init" = systemd ]; then service_file=/etc/systemd/system/sshguard-agent.service; else service_file=/etc/init.d/sshguard-agent; fi
    cp -p "$service_file" "$tmp/previous-service"
    cp -p "$exe" "$tmp/previous"
    if service_active; then was_active=true; fi
    rollback=true
    service_stop
    mv -f "$tmp/agent" "$exe"
    write_service
    service_start
    sleep 2
    service_active
    "$exe" -check -endpoint "$endpoint" -config "$config"
    rollback=false
    echo 'Agent updated; existing server identity and credentials preserved.'
    exit 0
fi
if ! id sshguard >/dev/null 2>&1; then
    if command -v useradd >/dev/null 2>&1; then useradd --system --user-group --no-create-home --shell /usr/sbin/nologin sshguard
    else addgroup -S sshguard; adduser -S -D -H -h /nonexistent -s /sbin/nologin -G sshguard sshguard; fi
fi
prepare_collection
mkdir -p /etc/sshguard
chown root:sshguard /etc/sshguard
chmod 0750 /etc/sshguard
printf '%s' "$enrollment" | "$tmp/agent" -enroll -endpoint "$endpoint" -config "$config"
unset enrollment
chown root:sshguard "$config"
chmod 0640 "$config"
if service_active; then service_stop; fi
mv -f "$tmp/agent" "$exe"
write_service
service_start
sleep 2
service_active
"$exe" -check -endpoint "$endpoint" -config "$config"
echo "Installed for $init. Durable SSH collection enabled; check collector health in the console."
