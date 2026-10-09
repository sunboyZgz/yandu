#!/usr/bin/env bash
# An explicit administrator emergency block. v0.1 has one trusted owner's tunnel.
set -euo pipefail
[[ $EUID == 0 && -f /etc/yandu/registry.json ]] || { echo '需要 root 和 Yandu 安装归属记录';exit 1; }
[[ $# == 2 && $1 == --site ]] || { echo '用法：sudo bash block-cloud.sh --site main';exit 1; }
YANDU_BLOCK_SITE=$2
[[ $YANDU_BLOCK_SITE =~ ^[a-z][a-z0-9-]{0,47}$ ]] || exit 1
python3 - "$YANDU_BLOCK_SITE" <<'PY'
import fcntl,json,pathlib,sys,os
base=pathlib.Path('/etc/yandu')
with (base/'apply.lock').open('a') as lock:
 fcntl.flock(lock,fcntl.LOCK_EX)
 r=json.loads((base/'registry.json').read_text());found=False
 for site in r['sites']:
  if site['id']==sys.argv[1]:site['blocked']=True;found=True
 if not found:sys.exit('站点不存在')
 tmp=base/'registry.block-next';tmp.write_text(json.dumps(r,indent=2));tmp.chmod(0o600);tmp.replace(base/'registry.json')
PY
systemctl disable --now yandu-frps
echo '资源入口已封禁，设备普通同步不能恢复。v0.1 共享的资源隧道已停止；业务网站继续运行。'
echo '解封需 root 管理员明确清除 registry.json 的 blocked 字段并核对配置后 enable --now yandu-frps。'
