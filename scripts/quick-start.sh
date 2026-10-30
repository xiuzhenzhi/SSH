#!/bin/sh
# Compatibility entry. All implementation lives in sshguard.sh.
set -eu
script_dir=$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd)
for entry in "$script_dir/sshguard.sh" "$script_dir/../sshguard.sh"; do
    if [ -f "$entry" ]; then exec sh "$entry" "$@"; fi
done
# Support old commands that download this file alone.
tmp=$(mktemp -d)
trap 'rm -rf -- "$tmp"' EXIT
trap 'exit 1' HUP INT TERM
curl --proto '=https' --proto-redir '=https' --tlsv1.2 --connect-timeout 15 --max-time 120 -fLsS https://raw.githubusercontent.com/xiuzhenzhi/SSH/main/sshguard.sh -o "$tmp/sshguard.sh"
sh "$tmp/sshguard.sh" "$@"
