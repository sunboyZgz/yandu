#!/usr/bin/env bash
set -euo pipefail
[[ $EUID == 0 ]] || { echo '需要 sudo';exit 1; }
[[ -f /etc/yandu/registry.json ]] || { echo '无安装归属记录，停止卸载';exit 1; }
echo '停止资源隧道，保留原网站、业务文件、受管拒绝路径及恢复记录。'
systemctl disable --now yandu-frps || true
python3 - <<'PY'
import json,pathlib,secrets
base=pathlib.Path('/etc/yandu');r=json.loads((base/'registry.json').read_text());s=json.loads((base/'state.json').read_text());g=secrets.token_hex(16);d=base/'nginx/generations'/g;d.mkdir(parents=True)
for site in r['sites']:
 site['blocked']=True
 text='# Yandu managed generation '+g+'; do not edit\n'
 for route in s['routes'].get(site['id'],[]):
  route['state']='disabled';text+=f"location ^~ {route['prefix']} {{\n add_header X-Yandu-Generation {g} always;\n add_header X-Content-Type-Options nosniff always;\n return 404;\n}}\n"
 (d/(site['id']+'.locations.conf')).write_text(text)
tmp=base/'nginx/uninstall-next';tmp.symlink_to('generations/'+g);old=(base/'nginx/current').readlink();tmp.replace(base/'nginx/current')
import subprocess
try:subprocess.run(['/usr/sbin/nginx','-t'],check=True);subprocess.run(['systemctl','reload','nginx'],check=True)
except subprocess.CalledProcessError:
 tmp.symlink_to(old);tmp.replace(base/'nginx/current');raise
s['generation']=g;(base/'state.json').write_text(json.dumps(s));(base/'registry.json').write_text(json.dumps(r))
PY
rm -f /etc/ssh/yandu_authorized_keys /etc/sudoers.d/yandu /etc/ssh/sshd_config.d/90-yandu.conf /etc/systemd/system/yandu-frps.service
/usr/sbin/sshd -t;systemctl reload ssh;systemctl daemon-reload
# Preserve root-owned configuration and TLS materials for an explicit recovery/removal decision.
rm -f /usr/local/lib/yandu/frps /usr/local/lib/yandu/yandu-cloud
echo '资源服务已卸载；/etc/yandu 和网站 include 保留，以维持停用资源 404。'
