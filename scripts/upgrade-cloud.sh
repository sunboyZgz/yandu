#!/usr/bin/env bash
set -Eeuo pipefail
umask 077
YANDU_CONFIG=/etc/yandu
YANDU_LIB=/usr/local/lib/yandu
YANDU_UNITS=/etc/systemd/system
YANDU_CLI_LINK=/usr/local/bin/yandu
YANDU_SOURCE="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
# Source checkouts prepare a temporary candidate; release packages already have it.
if [[ -f $YANDU_SOURCE/../go.mod && -d $YANDU_SOURCE/../cmd/yandu-cloud && -f $YANDU_SOURCE/prepare-cloud-upgrade.py ]];then
 for YANDU_PYTHON in python3 /usr/bin/python3 python;do
  if command -v "$YANDU_PYTHON" >/dev/null && "$YANDU_PYTHON" -c 'import struct,sys;sys.exit(not(sys.version_info >= (3,10) and struct.calcsize("P") == 8))' 2>/dev/null;then
   exec "$YANDU_PYTHON" "$YANDU_SOURCE/prepare-cloud-upgrade.py" "$@"
  fi
 done
 echo '源码升级需要 Python 3.10+（64 位）；请先准备环境：sudo bash ./setup-cloud-env.sh --allow-system-install' >&2
 exit 1
fi
YANDU_COMPONENT=cli YANDU_DRY=0 YANDU_ALLOW_DISCONNECT=0 YANDU_DRAIN=300
usage() {
 cat <<'HELP'
用法：sudo bash upgrade-cloud.sh [--component cli|gateway|tunnel|all] [--dry-run]
                                [--drain-timeout 秒数] [--allow-disconnect]
默认只更新 CLI，不重启服务；CLI 更新只需同目录的 yandu-cloud 二进制。
gateway：更新网关，默认最多等待当前传输 300 秒，期间新请求需重试。
tunnel：更新隧道组件，仅内容/配置变化时重启；活动隧道重启需 --allow-disconnect。
all：更新上述全部组件。证书、配对、项目与业务凭据保留。
HELP
}
while (($#));do
 case "$1" in
  --component) [[ $# -ge 2 ]] || { usage >&2;exit 1; };YANDU_COMPONENT=$2;shift 2;;
  --drain-timeout) [[ $# -ge 2 ]] || { usage >&2;exit 1; };YANDU_DRAIN=$2;shift 2;;
  --allow-disconnect) YANDU_ALLOW_DISCONNECT=1;shift;;
  --dry-run) YANDU_DRY=1;shift;;
  -h|--help) usage;exit 0;;
  *) echo "未知参数：$1" >&2;usage >&2;exit 1;;
 esac
done
case "$YANDU_COMPONENT" in cli|gateway|tunnel|all) ;; *) usage >&2;exit 1;; esac
[[ $YANDU_DRAIN =~ ^[0-9]{1,5}$ ]] && ((10#$YANDU_DRAIN > 0 && 10#$YANDU_DRAIN <= 86400)) || { echo 'drain-timeout 必须为 1–86400 秒' >&2;exit 1; }
YANDU_DRAIN=$((10#$YANDU_DRAIN))
[[ $EUID == 0 && ( -f $YANDU_CONFIG/registry.json || -f $YANDU_CONFIG/storage-owner.json ) ]] || { echo '需要已安装的云端环境和 sudo' >&2;exit 1; }
[[ $(uname -s) == Linux && $(uname -m) == x86_64 ]] || { echo '升级需要 Ubuntu x64' >&2;exit 1; }
. /etc/os-release
if [[ ${ID:-} != ubuntu || ! ${VERSION_ID:-} =~ ^[0-9]{2}\.[0-9]{2}$ ]] || ((10#${VERSION_ID/./} < 2404));then echo '升级需要 Ubuntu 24.04+ x64，未替换程序。' >&2;exit 1;fi
for YANDU_COMMAND in systemctl python3 flock sha256sum mktemp install cp mv ln;do
 command -v "$YANDU_COMMAND" >/dev/null || { echo "缺少命令：$YANDU_COMMAND" >&2;exit 1; }
done
[[ -n $(systemctl show --property=Version --value) ]] || { echo 'systemd 管理器不可用' >&2;exit 1; }
if [[ -e $YANDU_CLI_LINK || -L $YANDU_CLI_LINK ]];then
 [[ -L $YANDU_CLI_LINK && $(readlink "$YANDU_CLI_LINK") == "$YANDU_LIB/yandu-cloud" ]] || { echo "已有其他 $YANDU_CLI_LINK，停止升级" >&2;exit 1; }
fi
YANDU_CANDIDATE="$YANDU_SOURCE/yandu-cloud"
[[ -x $YANDU_CANDIDATE && -x $YANDU_LIB/yandu-cloud ]] || { echo '需要同目录的可执行 yandu-cloud，以及已安装的云端程序' >&2;exit 1; }
for YANDU_BINARY in yandu-cloud yandu-gateway yandu-tunnel frps;do
 if [[ -e $YANDU_LIB/$YANDU_BINARY || -L $YANDU_LIB/$YANDU_BINARY ]];then
  [[ -f $YANDU_LIB/$YANDU_BINARY && -x $YANDU_LIB/$YANDU_BINARY ]] || { echo "运行文件无效：$YANDU_LIB/$YANDU_BINARY" >&2;exit 1; }
 fi
done
"$YANDU_CANDIDATE" version >/dev/null
YANDU_UPDATE_CLI=0 YANDU_UPDATE_GATEWAY=0 YANDU_UPDATE_TUNNEL=0
case "$YANDU_COMPONENT" in
 cli) YANDU_UPDATE_CLI=1;;
 gateway) YANDU_UPDATE_GATEWAY=1;;
 tunnel) YANDU_UPDATE_TUNNEL=1;;
 all) YANDU_UPDATE_CLI=1;YANDU_UPDATE_TUNNEL=1;[[ ! -f $YANDU_CONFIG/storage-owner.json ]] || YANDU_UPDATE_GATEWAY=1;;
esac
YANDU_GATEWAY_UNIT="$YANDU_UNITS/yandu-gateway.service"
YANDU_TUNNEL_UNIT="$YANDU_UNITS/yandu-frps.service"
YANDU_GATEWAY_OLD="ExecStart=$YANDU_LIB/yandu-cloud storage-gateway --config $YANDU_CONFIG/gateway.json"
YANDU_GATEWAY_NEW="ExecStart=$YANDU_LIB/yandu-gateway storage-gateway --config $YANDU_CONFIG/gateway.json"
YANDU_TUNNEL_OLD="ExecStart=$YANDU_LIB/yandu-cloud tools run frps --config $YANDU_CONFIG/tools.json -- -c $YANDU_CONFIG/frps.toml"
YANDU_TUNNEL_NEW="ExecStart=$YANDU_LIB/yandu-tunnel tools run frps --config $YANDU_CONFIG/tools.json -- -c $YANDU_CONFIG/frps.toml"
# Only rewrite our known unit commands. Custom launchers require an explicit migration.
if [[ -f $YANDU_GATEWAY_UNIT ]];then
 grep -Fxq "$YANDU_GATEWAY_OLD" "$YANDU_GATEWAY_UNIT" || grep -Fxq "$YANDU_GATEWAY_NEW" "$YANDU_GATEWAY_UNIT" || { echo '网关使用自定义 ExecStart，请先核对启动配置；未替换程序。' >&2;exit 1; }
 if grep -Fxq "$YANDU_GATEWAY_OLD" "$YANDU_GATEWAY_UNIT" && [[ -e $YANDU_LIB/yandu-gateway ]] && ! cmp -s "$YANDU_LIB/yandu-cloud" "$YANDU_LIB/yandu-gateway";then echo '已有不同版本的网关运行文件，停止自动接管。' >&2;exit 1;fi
fi
if [[ -f $YANDU_TUNNEL_UNIT ]];then
 grep -Fxq "$YANDU_TUNNEL_OLD" "$YANDU_TUNNEL_UNIT" || grep -Fxq "$YANDU_TUNNEL_NEW" "$YANDU_TUNNEL_UNIT" || grep -Fxq "ExecStart=$YANDU_LIB/frps -c $YANDU_CONFIG/frps.toml" "$YANDU_TUNNEL_UNIT" || { echo '隧道使用自定义 ExecStart，请先核对启动配置；未替换程序。' >&2;exit 1; }
 if grep -Fxq "$YANDU_TUNNEL_OLD" "$YANDU_TUNNEL_UNIT" && [[ -e $YANDU_LIB/yandu-tunnel ]] && ! cmp -s "$YANDU_LIB/yandu-cloud" "$YANDU_LIB/yandu-tunnel";then echo '已有不同版本的隧道启动器，停止自动接管。' >&2;exit 1;fi
fi
if [[ $YANDU_UPDATE_GATEWAY == 1 ]];then
 [[ -f $YANDU_GATEWAY_UNIT && -f $YANDU_CONFIG/gateway.json ]] || { echo '当前部署没有存储网关' >&2;exit 1; }
 "$YANDU_CANDIDATE" storage-gateway --config "$YANDU_CONFIG/gateway.json" --check-config --shutdown-timeout "${YANDU_DRAIN}s"
fi
if [[ $YANDU_UPDATE_TUNNEL == 1 ]];then
 [[ -x $YANDU_SOURCE/bin/frps && -f $YANDU_TUNNEL_UNIT ]] || { echo '隧道更新需要发布包中的 bin/frps 和现有隧道服务' >&2;exit 1; }
 "$YANDU_CANDIDATE" tools prepare --check --role cloud --config "$YANDU_CONFIG/tools.json" --component-root "$YANDU_SOURCE" --tool "frps=$YANDU_SOURCE/bin/frps" >/dev/null
 "$YANDU_SOURCE/bin/frps" verify -c "$YANDU_CONFIG/frps.toml"
fi
if [[ $YANDU_DRY == 0 ]];then
 exec 9>"$YANDU_CONFIG/.upgrade.lock"
 flock -n 9 || { echo '另一个云端升级正在执行' >&2;exit 1; }
fi
fingerprint() {
 { sha256sum < "$1";sha256sum < "$2";printf '%s\n' "$3"; } | sha256sum | cut -d ' ' -f1
}
YANDU_GATEWAY_ACTIVE=0 YANDU_TUNNEL_ACTIVE=0 YANDU_GATEWAY_RESTART=0 YANDU_TUNNEL_RESTART=0 YANDU_BLOCKED=0
if [[ -f $YANDU_GATEWAY_UNIT ]] && systemctl is-active --quiet yandu-gateway;then YANDU_GATEWAY_ACTIVE=1;fi
if [[ -f $YANDU_TUNNEL_UNIT ]] && systemctl is-active --quiet yandu-frps;then YANDU_TUNNEL_ACTIVE=1;fi
if [[ $YANDU_UPDATE_GATEWAY == 1 ]];then
 YANDU_GATEWAY_DESIRED=$(fingerprint "$YANDU_CANDIDATE" "$YANDU_CONFIG/gateway.json" "${YANDU_DRAIN}s")
 YANDU_GATEWAY_APPLIED=$(cat "$YANDU_CONFIG/gateway-applied.sha256" 2>/dev/null || true)
 YANDU_CURRENT_GATEWAY="$YANDU_LIB/yandu-gateway"
 [[ -e $YANDU_CURRENT_GATEWAY ]] || YANDU_CURRENT_GATEWAY="$YANDU_LIB/yandu-cloud"
 if [[ $YANDU_GATEWAY_ACTIVE == 1 ]] && { [[ $YANDU_GATEWAY_DESIRED != "$YANDU_GATEWAY_APPLIED" ]] || ! cmp -s "$YANDU_CANDIDATE" "$YANDU_CURRENT_GATEWAY"; };then YANDU_GATEWAY_RESTART=1;fi
fi
if [[ $YANDU_UPDATE_TUNNEL == 1 ]];then
 YANDU_TUNNEL_DESIRED=$(fingerprint "$YANDU_SOURCE/bin/frps" "$YANDU_CONFIG/frps.toml" "$YANDU_LIB/frps")
 YANDU_TUNNEL_APPLIED=$(cat "$YANDU_CONFIG/tunnel-applied.sha256" 2>/dev/null || true)
 if [[ $YANDU_TUNNEL_ACTIVE == 1 ]] && { [[ $YANDU_TUNNEL_DESIRED != "$YANDU_TUNNEL_APPLIED" ]] || ! cmp -s "$YANDU_SOURCE/bin/frps" "$YANDU_LIB/frps"; };then YANDU_TUNNEL_RESTART=1;fi
 if [[ -f $YANDU_CONFIG/registry.json && ! -f $YANDU_CONFIG/storage-owner.json ]];then
  YANDU_BLOCKED=$(python3 - "$YANDU_CONFIG/registry.json" <<'PY'
import json,sys
print(int(any(site.get('blocked') for site in json.load(open(sys.argv[1]))['sites'])))
PY
)
 fi
fi
printf '更新范围：%s\n' "$YANDU_COMPONENT"
[[ $YANDU_UPDATE_CLI == 0 ]] || echo 'CLI：原子替换程序，不重启后台服务。'
if [[ $YANDU_UPDATE_GATEWAY == 1 ]];then
 if [[ $YANDU_GATEWAY_RESTART == 1 ]];then
  printf '网关：等待当前传输完成，最长 %s 秒；切换期间新请求需要重试。\n' "$YANDU_DRAIN"
  YANDU_RUNNING_GATEWAY="$YANDU_LIB/yandu-gateway"
  [[ -e $YANDU_RUNNING_GATEWAY ]] || YANDU_RUNNING_GATEWAY="$YANDU_LIB/yandu-cloud"
  YANDU_GATEWAY_HELP=$("$YANDU_RUNNING_GATEWAY" storage-gateway --help 2>&1 || true)
  if [[ $YANDU_GATEWAY_HELP != *shutdown-timeout* ]];then echo '注意：旧网关不支持等待传输完成，首次切换仍可能中断请求，请在维护窗口执行。';fi
 else
  echo '网关：运行版本未变，或当前未运行；本次不重启、不自动启动。'
 fi
fi
if [[ $YANDU_UPDATE_TUNNEL == 1 ]];then
 if [[ $YANDU_BLOCKED == 1 ]];then
  echo '隧道：保留云端封禁，停用隧道，不自动解封。'
 elif [[ $YANDU_TUNNEL_RESTART == 1 ]];then
  echo '隧道：程序或配置变化（旧部署首次更新也需建立运行记录），会断开现有连接后重连。'
 else
  echo '隧道：程序与配置未变，或当前未运行；本次不重启、不自动启动。'
 fi
fi
[[ $YANDU_DRY == 0 ]] || { echo '预览完成，未替换程序或操作服务。';exit 0; }
if [[ $YANDU_TUNNEL_RESTART == 1 && $YANDU_BLOCKED == 0 && $YANDU_ALLOW_DISCONNECT == 0 ]];then
 echo '尚未替换程序或重启服务。请在维护窗口加 --allow-disconnect 允许隧道重连，或只更新 cli/gateway。' >&2;exit 1
fi
install -d -m 0700 "$YANDU_CONFIG/backups"
YANDU_BACKUP=$(mktemp -d "$YANDU_CONFIG/backups/upgrade-$(date -u +%Y%m%dT%H%M%SZ)-XXXXXX")
YANDU_CHANGED_PATHS=() YANDU_OLD_EXISTED=() YANDU_TEMP_FILES=()
YANDU_RESTARTED_GATEWAY=0 YANDU_RESTARTED_TUNNEL=0
save_file() {
 local target=$1 i
 for i in "${!YANDU_CHANGED_PATHS[@]}";do [[ ${YANDU_CHANGED_PATHS[$i]} != "$target" ]] || return 0;done
 i=${#YANDU_CHANGED_PATHS[@]}
 if [[ -e $target || -L $target ]];then
  cp -a -- "$target" "$YANDU_BACKUP/$i"
  YANDU_OLD_EXISTED+=(1)
 else
  YANDU_OLD_EXISTED+=(0)
 fi
 YANDU_CHANGED_PATHS+=("$target")
 printf '%s\n' "$target" >> "$YANDU_BACKUP/paths.txt"
}
cleanup() { if ((${#YANDU_TEMP_FILES[@]}));then rm -f -- "${YANDU_TEMP_FILES[@]}";fi; }
rollback() {
 local status=$1 i target temp failed=0
 ((BASH_SUBSHELL == 0)) || exit "$status"
 trap - ERR INT TERM
 set +e
 echo "升级失败，正在从 $YANDU_BACKUP 回退本次修改。" >&2
 for ((i=${#YANDU_CHANGED_PATHS[@]}-1;i>=0;i--));do
  target=${YANDU_CHANGED_PATHS[$i]}
  if [[ ${YANDU_OLD_EXISTED[$i]} == 1 ]];then
   temp=$(mktemp "${target}.rollback.XXXXXX")
   rm -f -- "$temp"
   if ! cp -a -- "$YANDU_BACKUP/$i" "$temp" || ! mv -fT -- "$temp" "$target";then failed=1;fi
  else
   rm -f -- "$target" || failed=1
  fi
 done
 systemctl daemon-reload || failed=1
 if [[ $YANDU_RESTARTED_GATEWAY == 1 && $YANDU_GATEWAY_ACTIVE == 1 ]];then systemctl restart yandu-gateway && systemctl is-active --quiet yandu-gateway || failed=1;fi
 if [[ $YANDU_RESTARTED_TUNNEL == 1 && $YANDU_TUNNEL_ACTIVE == 1 && $YANDU_BLOCKED == 0 ]];then systemctl restart yandu-frps && systemctl is-active --quiet yandu-frps || failed=1;fi
 cleanup
 if [[ $failed == 0 ]];then echo '已恢复旧程序和启动配置。' >&2;else echo '自动回退未全部完成，请根据上述备份检查服务状态。' >&2;fi
 exit "$status"
}
trap 'rollback $?' ERR
trap 'rollback 130' INT
trap 'rollback 143' TERM
atomic_copy() {
 local source=$1 target=$2 mode=$3 temp
 if [[ -f $target ]] && cmp -s "$source" "$target";then return 0;fi
 save_file "$target"
 temp=$(mktemp "${target}.new.XXXXXX")
 YANDU_TEMP_FILES+=("$temp")
 install -m "$mode" -- "$source" "$temp"
 mv -fT -- "$temp" "$target"
}
atomic_text() {
 local target=$1 mode=$2 value=$3 temp
 if [[ -f $target ]] && cmp -s "$target" <(printf '%s\n' "$value");then return 0;fi
 save_file "$target"
 temp=$(mktemp "${target}.new.XXXXXX")
 YANDU_TEMP_FILES+=("$temp")
 printf '%s\n' "$value" > "$temp"
 chmod "$mode" "$temp"
 mv -fT -- "$temp" "$target"
}
# Pin the old running code before replacing the shared CLI path. Hard links keep
# its inode available without copying a live executable or restarting services.
pin_runtime() {
 local runtime=$1 unit=$2 old=$3 new=$4 temp
 [[ -f $unit ]] || return 0
 if [[ ! -e $YANDU_LIB/$runtime ]];then
  save_file "$YANDU_LIB/$runtime"
  ln -- "$YANDU_LIB/yandu-cloud" "$YANDU_LIB/$runtime"
 fi
 if grep -Fxq "$old" "$unit";then
  save_file "$unit"
  temp=$(mktemp "${unit}.new.XXXXXX")
  YANDU_TEMP_FILES+=("$temp")
  python3 - "$unit" "$temp" "$old" "$new" <<'PY'
from pathlib import Path
import sys
source,target,old,new=sys.argv[1:]
Path(target).write_text(''.join(new+'\n' if line.rstrip('\n')==old else line for line in Path(source).read_text().splitlines(keepends=True)))
PY
  chmod 0644 "$temp"
  mv -fT -- "$temp" "$unit"
 fi
}
pin_runtime yandu-gateway "$YANDU_GATEWAY_UNIT" "$YANDU_GATEWAY_OLD" "$YANDU_GATEWAY_NEW"
pin_runtime yandu-tunnel "$YANDU_TUNNEL_UNIT" "$YANDU_TUNNEL_OLD" "$YANDU_TUNNEL_NEW"
# Activate the pinned paths before changing the CLI. An unrelated automatic
# service restart during the transaction must still launch the old runtime.
systemctl daemon-reload
verify_runtime_command() {
 local service=$1 command=$2 effective
 effective=$(systemctl show --property=ExecStart --value "$service")
 # Check the manager's effective command, including drop-ins, rather than
 # assuming that editing the main unit changed what systemd will execute.
 [[ $effective == *"argv[]=$command ;"* ]] || { echo "服务 $service 的有效启动命令被其他配置覆盖，未替换程序。" >&2;return 1; }
}
if [[ -f $YANDU_GATEWAY_UNIT ]];then verify_runtime_command yandu-gateway "${YANDU_GATEWAY_NEW#ExecStart=}";fi
if [[ -f $YANDU_TUNNEL_UNIT ]];then
 if grep -Fxq "ExecStart=$YANDU_LIB/frps -c $YANDU_CONFIG/frps.toml" "$YANDU_TUNNEL_UNIT";then
  verify_runtime_command yandu-frps "$YANDU_LIB/frps -c $YANDU_CONFIG/frps.toml"
 else
  verify_runtime_command yandu-frps "${YANDU_TUNNEL_NEW#ExecStart=}"
 fi
fi
if [[ $YANDU_UPDATE_CLI == 1 ]];then atomic_copy "$YANDU_CANDIDATE" "$YANDU_LIB/yandu-cloud" 0755;fi
if [[ $YANDU_UPDATE_GATEWAY == 1 ]];then
 atomic_copy "$YANDU_CANDIDATE" "$YANDU_LIB/yandu-gateway" 0755
 install -d -m 0755 "$YANDU_GATEWAY_UNIT.d"
 atomic_text "$YANDU_GATEWAY_UNIT.d/20-yandu-drain.conf" 0644 "$(printf '[Service]\nEnvironment=YANDU_GATEWAY_SHUTDOWN_TIMEOUT=%ss\nTimeoutStopSec=%ss' "$YANDU_DRAIN" "$((YANDU_DRAIN+15))")"
fi
if [[ $YANDU_UPDATE_TUNNEL == 1 ]];then
 atomic_copy "$YANDU_CANDIDATE" "$YANDU_LIB/yandu-tunnel" 0755
 atomic_copy "$YANDU_SOURCE/bin/frps" "$YANDU_LIB/frps" 0755
 save_file "$YANDU_CONFIG/tools.json"
 "$YANDU_CANDIDATE" tools prepare --role cloud --config "$YANDU_CONFIG/tools.json" --component-root "$YANDU_LIB" --tool "frps=$YANDU_LIB/frps" >/dev/null
fi
systemctl daemon-reload
if [[ ! -L $YANDU_CLI_LINK ]];then
 install -d -m 0755 "$(dirname "$YANDU_CLI_LINK")"
 save_file "$YANDU_CLI_LINK"
 ln -s "$YANDU_LIB/yandu-cloud" "$YANDU_CLI_LINK"
fi
if [[ $YANDU_GATEWAY_RESTART == 1 ]];then
 YANDU_RESTARTED_GATEWAY=1
 systemctl restart yandu-gateway
 systemctl is-active --quiet yandu-gateway
 atomic_text "$YANDU_CONFIG/gateway-applied.sha256" 0600 "$YANDU_GATEWAY_DESIRED"
elif [[ $YANDU_UPDATE_GATEWAY == 1 && $YANDU_GATEWAY_ACTIVE == 0 ]];then
 atomic_text "$YANDU_CONFIG/gateway-applied.sha256" 0600 "$YANDU_GATEWAY_DESIRED"
fi
if [[ $YANDU_UPDATE_TUNNEL == 1 && $YANDU_BLOCKED == 1 ]];then
 systemctl disable --now yandu-frps
 atomic_text "$YANDU_CONFIG/tunnel-applied.sha256" 0600 "$YANDU_TUNNEL_DESIRED"
elif [[ $YANDU_TUNNEL_RESTART == 1 ]];then
 YANDU_RESTARTED_TUNNEL=1
 systemctl restart yandu-frps
 systemctl is-active --quiet yandu-frps
 atomic_text "$YANDU_CONFIG/tunnel-applied.sha256" 0600 "$YANDU_TUNNEL_DESIRED"
elif [[ $YANDU_UPDATE_TUNNEL == 1 && $YANDU_TUNNEL_ACTIVE == 0 ]];then
 atomic_text "$YANDU_CONFIG/tunnel-applied.sha256" 0600 "$YANDU_TUNNEL_DESIRED"
fi
trap - ERR INT TERM
cleanup
printf '更新完成。备份：%s\n' "$YANDU_BACKUP"
echo '配对、证书、项目和业务凭据已保留。未运行的服务不会因升级自动启动。'
