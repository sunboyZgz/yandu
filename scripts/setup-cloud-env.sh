#!/usr/bin/env bash
# Bootstrap the source installer without requiring Python or Go beforehand.
set -euo pipefail
YANDU_SCRIPTS="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
YANDU_ACTION=install
while (($#)); do
 case "$1" in
  --check) YANDU_ACTION=check; shift;;
  --dry-run) YANDU_ACTION=preview; shift;;
  --help|-h) echo '用法：sudo bash ./setup-cloud-env.sh [--check | --dry-run]'; exit 0;;
  *) echo "未知参数：$1" >&2; exit 1;;
 esac
done
[[ $(uname -s) == Linux && $(uname -m) == x86_64 ]] || { echo '云端环境准备需要 Ubuntu x64。' >&2; exit 1; }
. /etc/os-release
if [[ ${ID:-} != ubuntu || ! ${VERSION_ID:-} =~ ^[0-9]{2}\.[0-9]{2}$ ]] || (( 10#${VERSION_ID/./} < 2404 )); then
 echo '云端环境准备需要 Ubuntu 24.04 或更新版本。' >&2; exit 1
fi
for YANDU_COMMAND in apt-get dpkg-query systemctl useradd; do
 command -v "$YANDU_COMMAND" >/dev/null 2>&1 || { echo "缺少 $YANDU_COMMAND，请使用带 systemd 和 apt 的 Ubuntu 服务器。" >&2; exit 1; }
done
if [[ $YANDU_ACTION == install && $EUID != 0 ]]; then
 echo '请使用 sudo bash ./setup-cloud-env.sh 安装环境。' >&2; exit 1
fi
YANDU_MISSING=()
for YANDU_PACKAGE in python3 ca-certificates git acl; do
 if [[ $(dpkg-query -W -f='${Status}' "$YANDU_PACKAGE" 2>/dev/null || true) != 'install ok installed' ]]; then
  YANDU_MISSING+=("$YANDU_PACKAGE")
 fi
done
if ((${#YANDU_MISSING[@]})); then
 printf '需要补齐 Ubuntu 软件包：%s\n' "${YANDU_MISSING[*]}"
 if [[ $YANDU_ACTION == install ]]; then
  apt-get update
  DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends "${YANDU_MISSING[@]}"
 fi
fi
YANDU_PYTHON=
for YANDU_CANDIDATE in "$(command -v python3 || true)" /usr/bin/python3 "$(command -v python || true)"; do
 if [[ -n $YANDU_CANDIDATE && -x $YANDU_CANDIDATE ]] && "$YANDU_CANDIDATE" -c 'import sys; sys.exit(sys.version_info < (3, 10))' >/dev/null 2>&1; then
  YANDU_PYTHON=$YANDU_CANDIDATE; break
 fi
done
if [[ $YANDU_ACTION == preview ]]; then
 echo '预览：复用满足 go.mod 最低版本的 Go；缺少时从 go.dev 下载并校验，安装到 /usr/local/lib/yandu-env/。'
 echo '不会安装软件、下载文件或修改服务。'
 exit 0
fi
[[ -n $YANDU_PYTHON ]] || { echo '未找到 Python 3.10+；请检查 Ubuntu python3 软件包安装结果。' >&2; exit 1; }
if [[ $YANDU_ACTION == check && ${#YANDU_MISSING[@]} != 0 ]]; then
 echo '环境尚未就绪；运行 sudo bash ./setup-cloud-env.sh 补齐。' >&2; exit 1
fi
YANDU_ARGUMENTS=()
[[ $YANDU_ACTION != check ]] || YANDU_ARGUMENTS+=(--check)
exec "$YANDU_PYTHON" "$YANDU_SCRIPTS/cloud_env.py" "${YANDU_ARGUMENTS[@]}"
