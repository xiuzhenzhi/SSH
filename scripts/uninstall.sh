#!/bin/sh
# Standalone central-platform uninstaller, including legacy /opt/sshguard installs.
# Never removes Agent files, database services, or an existing web stack.
set -eu
umask 077
die() { printf '%s\n' "$*" >&2; exit 1; }
[ "$(id -u)" -eq 0 ] || die '请使用 sudo sh uninstall.sh。'
if [ ! -f /etc/systemd/system/sshguard-api.service ] && [ ! -e /opt/sshguard/sshguard-api ] && [ ! -L /opt/sshguard ]; then
    printf '未检测到原生安装的 SSHGuard 中央平台，无需卸载。\n'
    exit 0
fi
case "${1:-}" in
    --yes) [ "$#" -eq 1 ] || die '参数无效。' ;;
    '')
        printf '卸载 SSHGuard 中央平台：停止并移除服务与当前程序。\n数据库、配置、历史发行包及 Agent 均保留，Nginx/OpenResty 不会被卸载。\n继续？[y/N]: '
        IFS= read -r answer
        case "$answer" in y|Y) ;; *) printf '已取消。\n'; exit 0 ;; esac ;;
    *) die '用法：sudo sh uninstall.sh [--yes]' ;;
esac
unit=/etc/systemd/system/sshguard-api.service
for path in "$unit"; do
    [ ! -L "$path" ] || die "拒绝符号链接：$path"
done
target=''
if [ -L /opt/sshguard ]; then
    target=$(readlink -f /opt/sshguard) || die '无法解析程序路径。'
    case "$target" in /opt/sshguard-releases/*) ;; *) die '程序链接指向非标准路径，停止自动卸载。' ;; esac
elif [ -d /opt/sshguard ]; then
    target=/opt/sshguard
fi
if [ -n "$target" ]; then
    [ ! -L "$target/sshguard-api" ] || die '程序文件为符号链接，请人工检查。'
fi
if [ -f "$unit" ]; then
    printf '[1/2] 停止并移除中央平台服务…\n'
    grep -Fxq 'ExecStart=/opt/sshguard/sshguard-api' "$unit" || die '同名服务使用其他程序路径，停止自动卸载。'
elif command -v systemctl >/dev/null 2>&1 && systemctl is-active --quiet sshguard-api; then
    die '检测到非标准位置的中央服务，请先检查 systemctl cat sshguard-api。'
fi
mkdir /run/sshguard-deploy.lock 2>/dev/null || die '安装或更新正在进行，请稍后重试。'
trap 'rmdir /run/sshguard-deploy.lock' EXIT
if [ -f "$unit" ]; then
    systemctl disable --now sshguard-api
    rm -f "$unit"
    systemctl daemon-reload
    systemctl reset-failed sshguard-api 2>/dev/null || true
fi
printf '[2/2] 移除中央平台程序…\n'
if [ -n "$target" ]; then
    rm -f "$target/sshguard-api"
    rmdir "$target" 2>/dev/null || true
fi
if [ -L /opt/sshguard ]; then rm /opt/sshguard; fi
printf '\n中央平台已卸载。\n数据和配置保留在 /var/lib/sshguard-api、/etc/sshguard；外部数据库未改动。\n旧程序目录和历史发行包保留，可自行备份清理。\n已有反代请在面板中手动移除；Agent 和其他网站不受影响。\n'
