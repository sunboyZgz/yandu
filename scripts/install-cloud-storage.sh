#!/usr/bin/env bash
set -euo pipefail
YANDU_SOURCE="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
SERVER= DEVICE=owner DRY=0
while (($#));do case "$1" in --server) SERVER=$2;shift 2;; --device) DEVICE=$2;shift 2;; --dry-run) DRY=1;shift;; *) echo "未知参数 $1" >&2;exit 1;; esac;done
[[ $EUID == 0 && $(uname -s) == Linux && $(uname -m) == x86_64 && $SERVER =~ ^[a-zA-Z0-9.-]+$ && $DEVICE =~ ^[a-z][a-z0-9-]{0,47}$ ]] || { echo "需要 Ubuntu x64 root、--server IP或DNS，以及有效设备 ID" >&2;exit 1; }
. /etc/os-release
# Ubuntu release IDs use YY.MM; compare numerically instead of rejecting newer releases.
if [[ ${ID:-} != ubuntu || ! ${VERSION_ID:-} =~ ^[0-9]{2}\.[0-9]{2}$ ]] || (( 10#${VERSION_ID/./} < 2404 )); then
 printf '不支持的系统：%s %s。storage-only 自动安装需要 Ubuntu 24.04 或更新版本的 x64 系统。\n' "${ID:-unknown}" "${VERSION_ID:-unknown}" >&2
 exit 1
fi
YANDU_MISSING_COMMANDS=()
for YANDU_COMMAND in systemctl useradd install chown chmod id ln apt-get;do
 command -v "$YANDU_COMMAND" >/dev/null 2>&1 || YANDU_MISSING_COMMANDS+=("$YANDU_COMMAND")
done
if ((${#YANDU_MISSING_COMMANDS[@]}));then
 printf '缺少云端安装所需命令：%s。请准备带 systemd、用户管理和 apt 的 Ubuntu 服务器环境。\n' "${YANDU_MISSING_COMMANDS[*]}" >&2
 exit 1
fi
[[ $DRY == 1 ]] && { echo "storage-only：TLS frps :7000 → 回环 :19090 → 私有网关 :19091；无需 Nginx。";exit 0; }
show_pair_command() {
 printf '  sudo /usr/local/lib/yandu/yandu-cloud pair --scope storage-only --dir /etc/yandu --device %q --server %q --out %q\n' "$DEVICE" "$SERVER" "/etc/yandu/$DEVICE.yandu-profile" >&2
}
if [[ -e /usr/local/bin/yandu || -L /usr/local/bin/yandu ]];then
 [[ -L /usr/local/bin/yandu && $(readlink /usr/local/bin/yandu) == /usr/local/lib/yandu/yandu-cloud ]] || { echo '已有不属于云端 Yandu 的 /usr/local/bin/yandu，停止安装';exit 1; }
fi
[[ ! -e /etc/yandu ]] || {
 echo "已有安装；请使用维护流程扩展或升级，不覆盖现有配置" >&2
 if [[ -f /etc/yandu/gateway.json && ! -e /etc/yandu/storage-owner.json ]];then
  echo '检测到存储网关配置，但尚无配对归属记录。若上次在连接包口令处失败，可单独继续配对：' >&2
  show_pair_command
 fi
 exit 1
}
for account in yandu-frps yandu-gateway;do ! id "$account" >/dev/null 2>&1 || { echo "同名账号已存在，拒绝接管" >&2;exit 1; };done
if [[ ! -x $YANDU_SOURCE/yandu-cloud || ! -x $YANDU_SOURCE/bin/frps ]];then
 if [[ -f $YANDU_SOURCE/../go.mod && -d $YANDU_SOURCE/../cmd/yandu-cloud && -f $YANDU_SOURCE/prepare-cloud-install.py ]];then
  echo '检测到源码仓库，先准备 Ubuntu 云端环境…'
  bash "$YANDU_SOURCE/setup-cloud-env.sh"
  YANDU_PYTHON=
  for YANDU_CANDIDATE in "$(command -v python3 || true)" /usr/bin/python3 "$(command -v python || true)";do
   if [[ -n $YANDU_CANDIDATE && -x $YANDU_CANDIDATE ]] && "$YANDU_CANDIDATE" -c 'import sys; sys.exit(sys.version_info < (3, 10))' >/dev/null 2>&1;then YANDU_PYTHON=$YANDU_CANDIDATE;break;fi
  done
  [[ -n $YANDU_PYTHON ]] || { echo '未找到 Python 3.10+，环境准备未完成。' >&2;exit 1; }
  exec "$YANDU_PYTHON" "$YANDU_SOURCE/prepare-cloud-install.py" --server "$SERVER" --device "$DEVICE"
 fi
fi
[[ -x $YANDU_SOURCE/yandu-cloud && -x $YANDU_SOURCE/bin/frps ]] || {
 printf '缺少云端发布包组件：需要可执行的 %s/yandu-cloud 和 %s/bin/frps。\n' "$YANDU_SOURCE" "$YANDU_SOURCE" >&2
 echo '请使用完整 Linux 云端发布包，或在完整源码仓库根目录运行 ./install-cloud.sh 自动准备组件。' >&2
 exit 1
}
if ! command -v setfacl >/dev/null;then apt-get update;apt-get install -y acl;fi
install -d -m 0750 /etc/yandu /etc/yandu/tls
install -d -m 0755 /usr/local/lib/yandu
install -m 0755 "$YANDU_SOURCE/yandu-cloud" /usr/local/lib/yandu/yandu-cloud
install -m 0755 "$YANDU_SOURCE/bin/frps" /usr/local/lib/yandu/frps
install -d -m 0755 /usr/local/bin
ln -sfn /usr/local/lib/yandu/yandu-cloud /usr/local/bin/yandu
/usr/local/lib/yandu/yandu-cloud tools prepare --role cloud --config /etc/yandu/tools.json --component-root /usr/local/lib/yandu
/usr/local/lib/yandu/yandu-cloud storage-init --server "$SERVER"
useradd --system --user-group --no-create-home --shell /usr/sbin/nologin yandu-frps
useradd --system --user-group --no-create-home --shell /usr/sbin/nologin yandu-gateway
chown root:yandu-frps /etc/yandu /etc/yandu/tls /etc/yandu/frps.toml /etc/yandu/tls/server.key /etc/yandu/tls/server.crt
chmod 0640 /etc/yandu/frps.toml /etc/yandu/tls/server.key
chmod 0600 /etc/yandu/tls/ca.key /etc/yandu/tunnel.token
chmod 0644 /etc/yandu/gateway.json
# gateway needs traverse permission, never tunnel secrets.
setfacl -m u:yandu-gateway:x /etc/yandu
for component in frps gateway;do
 if [[ $component == frps ]];then YANDU_EXEC='/usr/local/lib/yandu/yandu-cloud tools run frps --config /etc/yandu/tools.json -- -c /etc/yandu/frps.toml';else YANDU_EXEC='/usr/local/lib/yandu/yandu-cloud storage-gateway --config /etc/yandu/gateway.json';fi
 cat > "/etc/systemd/system/yandu-$component.service" <<UNIT
[Unit]
Description=Yandu $component
After=network-online.target
Wants=network-online.target
[Service]
User=yandu-$component
Group=yandu-$component
ExecStart=$YANDU_EXEC
Restart=always
RestartSec=3
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=true
PrivateTmp=true
[Install]
WantedBy=multi-user.target
UNIT
done
/usr/local/lib/yandu/yandu-cloud tools run frps --config /etc/yandu/tools.json -- verify -c /etc/yandu/frps.toml
systemctl daemon-reload
systemctl enable --now yandu-frps yandu-gateway
cd /etc/yandu
if ! /usr/local/lib/yandu/yandu-cloud pair --scope storage-only --device "$DEVICE" --server "$SERVER" --out "$DEVICE.yandu-profile";then
 echo '云端服务已安装并启用，连接包生成未完成。若上方是口令错误，可用以下命令继续配对：' >&2
 show_pair_command
 exit 1
fi
echo '安装完成：仅向本地设备开放 7000/TCP；19090 和 19091 保持回环监听。'
