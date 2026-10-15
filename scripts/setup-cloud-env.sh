#!/usr/bin/env bash
# Bootstrap the source installer without requiring Python or Go beforehand.
set -euo pipefail
YANDU_SCRIPTS="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
YANDU_ACTION=install
YANDU_ALLOW_SYSTEM=0
while (($#)); do
 case "$1" in
  --check) YANDU_ACTION=check; shift;;
  --dry-run) YANDU_ACTION=preview; shift;;
  --allow-system-install) YANDU_ALLOW_SYSTEM=1; shift;;
  --help|-h) echo '用法：bash ./setup-cloud-env.sh [--check | --dry-run | --allow-system-install]；默认只检查，不安装系统工具。'; exit 0;;
  *) echo "未知参数：$1" >&2; exit 1;;
 esac
done
[[ $YANDU_ALLOW_SYSTEM == 0 || $YANDU_ACTION == install ]] || { echo '--check / --dry-run 不能与 --allow-system-install 同时使用。' >&2;exit 1; }
[[ $(uname -s) == Linux && $(uname -m) == x86_64 ]] || { echo '云端环境准备需要 Ubuntu x64。' >&2; exit 1; }
. /etc/os-release
if [[ ${ID:-} != ubuntu || ! ${VERSION_ID:-} =~ ^[0-9]{2}\.[0-9]{2}$ ]] || (( 10#${VERSION_ID/./} < 2404 )); then
 echo '云端环境准备需要 Ubuntu 24.04 或更新版本。' >&2; exit 1
fi
for YANDU_COMMAND in apt-get dpkg-query systemctl useradd; do
 command -v "$YANDU_COMMAND" >/dev/null 2>&1 || { echo "缺少 $YANDU_COMMAND，请使用带 systemd 和 apt 的 Ubuntu 服务器。" >&2; exit 1; }
done
if ! YANDU_SYSTEMD_INFO=$(systemctl show --property=Version --value 2>&1) || [[ -z $YANDU_SYSTEMD_INFO ]];then
 printf 'systemd 管理器不可用，不能安装云端服务；未修改系统：%s\n' "$YANDU_SYSTEMD_INFO" >&2;exit 1
fi
if [[ $YANDU_ALLOW_SYSTEM == 1 && $EUID != 0 ]]; then
 echo '安装系统工具需要 sudo bash ./setup-cloud-env.sh --allow-system-install。' >&2; exit 1
fi
echo '检查系统支持：Ubuntu x64，Python 3.10+ 64 位，Go 最低版本取自 go.mod。'
probe_python() {
 YANDU_PYTHON=
 YANDU_PROBE_FAILED=0
 for YANDU_CANDIDATE in "$(command -v python3 || true)" /usr/bin/python3 "$(command -v python || true)"; do
  [[ -n $YANDU_CANDIDATE && -e $YANDU_CANDIDATE ]] || continue
  if [[ ! -x $YANDU_CANDIDATE ]];then printf '已有 Python 不可执行：%s\n' "$YANDU_CANDIDATE" >&2;YANDU_PROBE_FAILED=1;continue;fi
  if YANDU_PYTHON_INFO=$("$YANDU_CANDIDATE" -I -c 'import sys,struct; print(sys.executable); print(sys.version.split()[0]); print(struct.calcsize("P")*8); sys.exit(0 if sys.version_info >= (3,10) and struct.calcsize("P")==8 else 10)' 2>&1);then
   printf '复用 Python：%s\n' "$YANDU_PYTHON_INFO"
   YANDU_PYTHON=$YANDU_CANDIDATE
   return 0
  else
   YANDU_PROBE_CODE=$?
   printf 'Python 检查未通过：%s（退出码 %s）\n%s\n' "$YANDU_CANDIDATE" "$YANDU_PROBE_CODE" "$YANDU_PYTHON_INFO" >&2
   [[ $YANDU_PROBE_CODE == 10 ]] || YANDU_PROBE_FAILED=1
  fi
 done
}
probe_python
if [[ -z $YANDU_PYTHON && $YANDU_PROBE_FAILED == 1 ]];then
 echo '现有 Python 检测失败，不按缺失处理；未调用系统安装器。' >&2;exit 1
fi
if [[ -n $YANDU_PYTHON ]];then
 # Discover existing Go failures before apt is permitted to change the host.
 "$YANDU_PYTHON" "$YANDU_SCRIPTS/cloud_env.py" --dry-run
fi
YANDU_MISSING=()
[[ -n $YANDU_PYTHON ]] || YANDU_MISSING+=(python3)
for YANDU_PACKAGE in ca-certificates git acl; do
 if YANDU_PACKAGE_STATUS=$(dpkg-query -W -f='${Status}' "$YANDU_PACKAGE" 2>&1);then
  [[ $YANDU_PACKAGE_STATUS == 'install ok installed' ]] || YANDU_MISSING+=("$YANDU_PACKAGE")
 else
  YANDU_PACKAGE_CODE=$?
  if [[ $YANDU_PACKAGE_CODE == 1 ]];then YANDU_MISSING+=("$YANDU_PACKAGE");else printf '软件包状态检测失败，不按缺失处理：%s（退出码 %s）%s\n' "$YANDU_PACKAGE" "$YANDU_PACKAGE_CODE" "$YANDU_PACKAGE_STATUS" >&2;exit 1;fi
 fi
done
if ((${#YANDU_MISSING[@]})); then
 printf '需要补齐 Ubuntu 软件包：%s\n' "${YANDU_MISSING[*]}"
 if [[ $YANDU_ACTION == install && $YANDU_ALLOW_SYSTEM == 1 ]]; then
  echo '已显式允许系统安装：apt 可能更新软件包及系统依赖；私有 Go 写入 /usr/local/lib/yandu-env/，不替换系统 Go，不修改持久 PATH。'
  apt-get update
  DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends "${YANDU_MISSING[@]}"
 elif [[ $YANDU_ACTION != preview ]];then
  echo '默认不安装系统软件包。请准备上述依赖，或显式加 --allow-system-install。' >&2;exit 1
 fi
fi
if [[ $YANDU_ACTION == preview ]]; then
 [[ -n $YANDU_PYTHON ]] || echo '预览：Python 尚不可用，未执行 Go 探针；Go 最低版本由 go.mod 决定。'
 echo '不会安装软件、下载文件或修改服务。'
 exit 0
fi
[[ -n $YANDU_PYTHON ]] || probe_python
[[ -n $YANDU_PYTHON ]] || { echo '未找到 Python 3.10+；请检查 Ubuntu python3 软件包安装结果。' >&2; exit 1; }
YANDU_ARGUMENTS=()
[[ $YANDU_ACTION != check ]] || YANDU_ARGUMENTS+=(--check)
[[ $YANDU_ALLOW_SYSTEM == 0 ]] || YANDU_ARGUMENTS+=(--allow-system-install)
echo '运行时合格则复用；默认不调用 apt，不下载到系统目录。'
exec "$YANDU_PYTHON" "$YANDU_SCRIPTS/cloud_env.py" "${YANDU_ARGUMENTS[@]}"
