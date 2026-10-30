#!/bin/sh
# Download as the invoking user; elevate only the verified deployment step.
set -eu
case "${1:-}" in
    --help|-h)
        printf 'SSHGuard 主控\n\n安装：sh scripts/quick-start.sh [--port 8080]\n更新到最新正式版：sh scripts/quick-start.sh --update\n指定版本：SSHGUARD_VERSION=v版本号 sh scripts/quick-start.sh --update\n管理：--status / --logs / --restart / --start / --stop / --setup-link / --uninstall\n\n默认 IP + 端口、SQLite，无需域名、Docker 或 Nginx。\n支持 Ubuntu / Debian + systemd，amd64 / arm64。\n'
        exit 0 ;;
esac
if [ "$#" -eq 0 ] && [ -t 0 ]; then
    printf '\n========== SSHGuard ==========\n'
    default_choice=1
    if [ -f /opt/sshguard/deploy.sh ]; then printf '状态：已安装\n'; default_choice=9; else printf '状态：未安装\n'; fi
    printf '1) 安装主控\n2) 查看状态与访问地址\n3) 查看日志\n4) 重启\n5) 启动\n6) 停止\n7) 卸载主控（保留数据）\n8) 查看初始化链接\n9) 更新主控（下载最新正式版）\n0) 退出\n请选择 [默认 %s]: ' "$default_choice"
    IFS= read -r choice
    case "${choice:-$default_choice}" in
        1) set -- --install ;; 9) set -- --update ;;
        2) set -- --status ;; 3) set -- --logs ;; 4) set -- --restart ;;
        5) set -- --start ;; 6) set -- --stop ;; 7) set -- --uninstall ;; 8) set -- --setup-link ;; 0) exit 0 ;;
        *) echo '无效选项。' >&2; exit 1 ;;
    esac
fi
if [ "$#" -eq 0 ] && [ -f /opt/sshguard/deploy.sh ]; then set -- --update; fi
case "${1:-}" in
    --status|--logs|--restart|--start|--stop|--setup-link|--uninstall)
        [ -f /opt/sshguard/deploy.sh ] || { echo 'No installed SSHGuard management script found.' >&2; exit 1; }
        if [ "$(id -u)" -eq 0 ]; then exec sh /opt/sshguard/deploy.sh "$@";
        else exec sudo sh /opt/sshguard/deploy.sh "$@"; fi ;;
esac
version=${SSHGUARD_VERSION:-v0.3.11}
for arg in "$@"; do
    if [ "$arg" = --update ] && [ -z "${SSHGUARD_VERSION:-}" ]; then version=latest; fi
done
valid_version() {
    case "$1" in ''|*[!A-Za-z0-9.+_-]*) return 1 ;; esac
    printf '%s' "$1" | grep -Eq '^v[0-9][A-Za-z0-9.+_-]{0,31}$'
}
[ "$version" = latest ] || valid_version "$version" || { echo 'Invalid SSHGUARD_VERSION' >&2; exit 1; }
case "$(uname -m)" in x86_64) arch=amd64 ;; aarch64|arm64) arch=arm64 ;; *) echo 'Only Linux amd64/arm64 supported.' >&2; exit 1 ;; esac
[ "$(uname -s)" = Linux ] || { echo 'Run on the Ubuntu/Debian central server.' >&2; exit 1; }
for tool in sha256sum tar; do command -v "$tool" >/dev/null 2>&1 || { echo "Required command: $tool" >&2; exit 1; }; done
github_cli=false
if command -v gh >/dev/null 2>&1 && gh auth status >/dev/null 2>&1; then github_cli=true; fi
if ! "$github_cli"; then command -v curl >/dev/null 2>&1 || { echo 'Required command: curl (or authenticated gh)' >&2; exit 1; }; fi
github_curl() { curl --proto '=https' --proto-redir '=https' --tlsv1.2 --connect-timeout 15 --max-time 300 --retry 2 -fLsS "$@"; }
if [ "$version" = latest ]; then
    printf '正在查询 SSHGuard 最新正式版…\n'
    if "$github_cli"; then
        version=$(gh release view --repo xiuzhenzhi/SSH --json tagName --jq .tagName) || { echo '无法查询最新发行版，请检查 GitHub 登录和仓库访问权限。' >&2; exit 1; }
    else
        latest_url=$(github_curl --head --output /dev/null --write-out '%{url_effective}' https://github.com/xiuzhenzhi/SSH/releases/latest) || { echo '无法查询最新发行版，请检查网络；私有仓库请先执行 gh auth login。' >&2; exit 1; }
        case "$latest_url" in
            https://github.com/xiuzhenzhi/SSH/releases/tag/*) version=${latest_url##*/} ;;
            *) echo '无法解析最新发行版，请检查仓库是否已有正式 Release；私有仓库请先执行 gh auth login。' >&2; exit 1 ;;
        esac
    fi
    valid_version "$version" || { echo 'Invalid release version returned by GitHub.' >&2; exit 1; }
fi
printf '下载并校验 SSHGuard %s（%s）\n' "$version" "$arch"
tmp=$(mktemp -d)
trap 'rm -rf -- "$tmp"' EXIT
trap 'exit 1' HUP INT TERM
asset="sshguard-linux-$arch.tar.gz"
if "$github_cli"; then
    gh release download "$version" --repo xiuzhenzhi/SSH --pattern "$asset" --pattern SHA256SUMS --dir "$tmp"
else
    base="https://github.com/xiuzhenzhi/SSH/releases/download/$version"
    for name in "$asset" SHA256SUMS; do
        github_curl "$base/$name" -o "$tmp/$name" || {
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
bundle="$tmp/sshguard-linux-$arch"
[ "$(cat "$bundle/VERSION")" = "${version#v}" ] || { echo 'Release VERSION does not match the requested tag.' >&2; exit 1; }
if [ "$(id -u)" -eq 0 ]; then sh "$bundle/deploy.sh" "$@"
else sudo sh "$bundle/deploy.sh" "$@"; fi
