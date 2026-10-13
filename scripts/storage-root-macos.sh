#!/bin/bash
set -euo pipefail
[[ $EUID == 0 && ${1:-} =~ ^[a-z][a-z0-9-]{0,47}$ ]] || { echo 'sudo storage-root-macos.sh 项目ID';exit 1; }
YANDU_ROOT="/Library/Application Support/Yandu/Storage/$1"
[[ ! -e $YANDU_ROOT ]] || { echo '目录已存在，拒绝覆盖';exit 1; }
install -d -o _yandu -g _yandusftpgo -m 0750 "$YANDU_ROOT"
install -d -o _yandusftpgo -g _yandu -m 0750 "$YANDU_ROOT/files"
install -d -o _yandu -g _yandu -m 0700 "$YANDU_ROOT/temporary" "$YANDU_ROOT/temporary/uploads" "$YANDU_ROOT/temporary/jobs"
echo "$YANDU_ROOT"
