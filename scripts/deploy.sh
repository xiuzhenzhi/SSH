#!/bin/sh
# Compatibility entry. All implementation lives in sshguard.sh.
set -eu
case "${1:-}" in --update-latest) shift; exec sh "$(dirname "$0")/update.sh" "$@" ;; esac
script_dir=$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd)
for entry in "$script_dir/sshguard.sh" "$script_dir/../sshguard.sh"; do
    if [ -f "$entry" ]; then exec sh "$entry" --deploy "$@"; fi
done
# Support old commands that download this file alone.
tmp=$(mktemp -d)
trap 'rm -rf -- "$tmp"' EXIT
trap 'exit 1' HUP INT TERM
curl --proto '=https' --proto-redir '=https' --tlsv1.2 --connect-timeout 15 --max-time 120 -fLsS https://raw.githubusercontent.com/xiuzhenzhi/SSHGuard/main/sshguard.sh -o "$tmp/sshguard.sh"
sh "$tmp/sshguard.sh" --deploy "$@"
