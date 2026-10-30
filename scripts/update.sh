#!/bin/sh
# Works both in the source scripts directory and beside an installed release.
set -eu
case "${1:-}" in
    --help|-h)
        printf 'SSHGuard 一键更新主控\n\n用法：sh update.sh\n默认下载最新正式版，校验后更新，保留配置和数据。\n指定版本：SSHGUARD_VERSION=v版本号 sh update.sh\n私有仓库请先以当前用户执行 gh auth login。\n'
        exit 0 ;;
esac
script_dir=$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd)
exec sh "$script_dir/quick-start.sh" --update "$@"
