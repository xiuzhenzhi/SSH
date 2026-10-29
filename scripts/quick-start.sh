#!/bin/sh
# Download as the invoking user; elevate only the verified deployment step.
set -eu
case "${1:-}" in
    --help|-h)
        printf 'SSHGuard 主控\n\n安装：sh scripts/quick-start.sh [--port 8080]\n更新：sh scripts/quick-start.sh --update\n管理：--status / --logs / --restart / --start / --stop / --setup-link / --uninstall\n\n默认 IP + 端口、SQLite，无需域名、Docker 或 Nginx。\n支持 Ubuntu / Debian + systemd，amd64 / arm64。\n'
        exit 0 ;;
esac
if [ "$#" -eq 0 ] && [ -t 0 ]; then
    printf '\n========== SSHGuard ==========\n'
    if [ -f /opt/sshguard/deploy.sh ]; then printf '状态：已安装\n'; else printf '状态：未安装\n'; fi
    printf '1) 安装 / 更新主控\n2) 查看状态与访问地址\n3) 查看日志\n4) 重启\n5) 启动\n6) 停止\n7) 卸载主控（保留数据）\n8) 查看初始化链接\n0) 退出\n请选择 [默认 1]: '
    IFS= read -r choice
    case "${choice:-1}" in
        1) if [ -f /opt/sshguard/deploy.sh ]; then set -- --update; else set -- --install; fi ;;
        2) set -- --status ;; 3) set -- --logs ;; 4) set -- --restart ;;
        5) set -- --start ;; 6) set -- --stop ;; 7) set -- --uninstall ;; 8) set -- --setup-link ;; 0) exit 0 ;;
        *) echo '无效选项。' >&2; exit 1 ;;
    esac
fi
case "${1:-}" in
    --status|--logs|--restart|--start|--stop|--setup-link|--uninstall)
        [ -f /opt/sshguard/deploy.sh ] || { echo 'No installed SSHGuard management script found.' >&2; exit 1; }
        if [ "$(id -u)" -eq 0 ]; then exec sh /opt/sshguard/deploy.sh "$@";
        else exec sudo sh /opt/sshguard/deploy.sh "$@"; fi ;;
esac
version=${SSHGUARD_VERSION:-v0.3.9}
printf '%s' "$version" | grep -Eq '^v[0-9][A-Za-z0-9.+_-]{0,31}$' || { echo 'Invalid SSHGUARD_VERSION' >&2; exit 1; }
case "$(uname -m)" in x86_64) arch=amd64 ;; aarch64|arm64) arch=arm64 ;; *) echo 'Only Linux amd64/arm64 supported.' >&2; exit 1 ;; esac
[ "$(uname -s)" = Linux ] || { echo 'Run on the Ubuntu/Debian central server.' >&2; exit 1; }
command -v sha256sum >/dev/null 2>&1
tmp=$(mktemp -d)
trap 'rm -rf -- "$tmp"' EXIT
asset="sshguard-linux-$arch.tar.gz"
if command -v gh >/dev/null 2>&1 && gh auth status >/dev/null 2>&1; then
    gh release download "$version" --repo xiuzhenzhi/SSH --pattern "$asset" --pattern SHA256SUMS --dir "$tmp"
else
    base="https://github.com/xiuzhenzhi/SSH/releases/download/$version"
    for name in "$asset" SHA256SUMS; do
        curl --proto '=https' --proto-redir '=https' --tlsv1.2 --connect-timeout 15 --max-time 300 --retry 2 -fLsS "$base/$name" -o "$tmp/$name" || {
            echo 'Download failed. For this private repository, install GitHub CLI and run gh auth login with repository access, then retry. No token is needed on Agents.' >&2
            exit 1
        }
    done
fi
(
    cd "$tmp"
    awk -v name="$asset" '$2==name {print; n++} END {if(n!=1)exit 1}' SHA256SUMS > selected.sha256
    sha256sum -c selected.sha256
    tar -xzf "$asset"
)
if [ "$(id -u)" -eq 0 ]; then sh "$tmp/sshguard-linux-$arch/deploy.sh" "$@"
else sudo sh "$tmp/sshguard-linux-$arch/deploy.sh" "$@"; fi
