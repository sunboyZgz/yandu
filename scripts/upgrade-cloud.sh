#!/usr/bin/env bash
set -euo pipefail
[[ $EUID == 0 && ( -f /etc/yandu/registry.json || -f /etc/yandu/storage-owner.json ) ]] || { echo '需要已安装的云端环境和 sudo';exit 1; }
if [[ -e /usr/local/bin/yandu || -L /usr/local/bin/yandu ]];then
 [[ -L /usr/local/bin/yandu && $(readlink /usr/local/bin/yandu) == /usr/local/lib/yandu/yandu-cloud ]] || { echo '已有其他 /usr/local/bin/yandu，停止升级';exit 1; }
fi
YANDU_SOURCE="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
[[ -f $YANDU_SOURCE/yandu-cloud && -f $YANDU_SOURCE/bin/frps ]] || { echo '需要完整发布包';exit 1; }
YANDU_BACKUP="/etc/yandu/backups/upgrade-$(date +%s)"
mkdir -p "$YANDU_BACKUP";chmod 0700 "$YANDU_BACKUP"
cp /usr/local/lib/yandu/yandu-cloud /usr/local/lib/yandu/frps "$YANDU_BACKUP/"
cp /etc/systemd/system/yandu-frps.service "$YANDU_BACKUP/yandu-frps.service"
[[ ! -f /etc/yandu/tools.json ]] || cp /etc/yandu/tools.json "$YANDU_BACKUP/tools.json"
yandu_restore_upgrade() {
 cp "$YANDU_BACKUP/yandu-cloud" "$YANDU_BACKUP/frps" /usr/local/lib/yandu/
 cp "$YANDU_BACKUP/yandu-frps.service" /etc/systemd/system/yandu-frps.service
 if [[ -f $YANDU_BACKUP/tools.json ]];then cp "$YANDU_BACKUP/tools.json" /etc/yandu/tools.json;else rm -f /etc/yandu/tools.json;fi
 systemctl daemon-reload
}
install -m 0755 "$YANDU_SOURCE/yandu-cloud" /usr/local/lib/yandu/yandu-cloud
install -m 0755 "$YANDU_SOURCE/bin/frps" /usr/local/lib/yandu/frps
if ! /usr/local/lib/yandu/yandu-cloud tools prepare --role cloud --config /etc/yandu/tools.json --component-root /usr/local/lib/yandu || ! /usr/local/lib/yandu/yandu-cloud tools run frps --config /etc/yandu/tools.json -- verify -c /etc/yandu/frps.toml;then yandu_restore_upgrade;exit 1;fi
sed -i 's#^ExecStart=/usr/local/lib/yandu/frps -c /etc/yandu/frps.toml$#ExecStart=/usr/local/lib/yandu/yandu-cloud tools run frps --config /etc/yandu/tools.json -- -c /etc/yandu/frps.toml#' /etc/systemd/system/yandu-frps.service
systemctl daemon-reload
install -d -m 0755 /usr/local/bin
ln -sfn /usr/local/lib/yandu/yandu-cloud /usr/local/bin/yandu
if [[ -f /etc/yandu/storage-owner.json ]];then
 if ! systemctl restart yandu-frps yandu-gateway || ! systemctl is-active --quiet yandu-frps || ! systemctl is-active --quiet yandu-gateway;then yandu_restore_upgrade;systemctl restart yandu-frps yandu-gateway;exit 1;fi
 echo '通用存储网关与隧道已升级，配对和凭据保留。';exit 0
fi
if ! python3 -c 'import json,sys;sys.exit(1 if any(s.get("blocked") for s in json.load(open("/etc/yandu/registry.json"))["sites"]) else 0)';then systemctl disable --now yandu-frps;echo '组件已升级，保留云端封禁';exit 0;fi
if ! systemctl restart yandu-frps || ! systemctl is-active --quiet yandu-frps;then yandu_restore_upgrade;systemctl restart yandu-frps;exit 1;fi
echo '组件升级完成，项目与凭据保持原样；隧道重连期间资源会短暂受影响。'
