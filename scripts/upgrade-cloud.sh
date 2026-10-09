#!/usr/bin/env bash
set -euo pipefail
[[ $EUID == 0 && -f /etc/yandu/registry.json ]] || { echo '需要已安装的云端环境和 sudo';exit 1; }
YANDU_SOURCE="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
[[ -f $YANDU_SOURCE/yandu-cloud && -f $YANDU_SOURCE/bin/frps ]] || { echo '需要完整发布包';exit 1; }
YANDU_BACKUP="/etc/yandu/backups/upgrade-$(date +%s)"
mkdir -p "$YANDU_BACKUP";chmod 0700 "$YANDU_BACKUP"
cp /usr/local/lib/yandu/yandu-cloud /usr/local/lib/yandu/frps "$YANDU_BACKUP/"
install -m 0755 "$YANDU_SOURCE/yandu-cloud" /usr/local/lib/yandu/yandu-cloud
install -m 0755 "$YANDU_SOURCE/bin/frps" /usr/local/lib/yandu/frps
if ! /usr/local/lib/yandu/frps verify -c /etc/yandu/frps.toml;then cp "$YANDU_BACKUP/"* /usr/local/lib/yandu/;exit 1;fi
if ! python3 -c 'import json,sys;sys.exit(1 if any(s.get("blocked") for s in json.load(open("/etc/yandu/registry.json"))["sites"]) else 0)';then systemctl disable --now yandu-frps;echo '组件已升级，保留云端封禁';exit 0;fi
systemctl restart yandu-frps
if ! systemctl is-active --quiet yandu-frps;then cp "$YANDU_BACKUP/"* /usr/local/lib/yandu/;systemctl restart yandu-frps;exit 1;fi
echo '组件升级完成，项目与凭据保持原样；隧道重连期间资源会短暂受影响。'
