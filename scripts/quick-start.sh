#!/bin/sh
# Download as the invoking user; elevate only the verified deployment step.
set -eu
case "${1:-}" in
    --status|--logs|--restart)
        [ -f /opt/sshguard/deploy.sh ] || { echo 'No installed SSHGuard management script found.' >&2; exit 1; }
        if [ "$(id -u)" -eq 0 ]; then exec sh /opt/sshguard/deploy.sh "$@";
        else exec sudo sh /opt/sshguard/deploy.sh "$@"; fi ;;
esac
version=${SSHGUARD_VERSION:-v0.3.2}
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
    awk -v name="$asset" '$2==name {print}' SHA256SUMS | sha256sum -c -
    tar -xzf "$asset"
)
if [ "$(id -u)" -eq 0 ]; then sh "$tmp/sshguard-linux-$arch/deploy.sh" "$@"
else sudo sh "$tmp/sshguard-linux-$arch/deploy.sh" "$@"; fi
