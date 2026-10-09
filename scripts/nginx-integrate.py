#!/usr/bin/env python3
"""Conservative native Nginx integration. Never selects a server by position."""
import argparse, difflib, os, pathlib, re, shutil, subprocess, sys, tempfile, time
p=argparse.ArgumentParser();p.add_argument('--config',required=True);p.add_argument('--domain',required=True);p.add_argument('--site-id',required=True);p.add_argument('--dry-run',action='store_true');a=p.parse_args()
f=pathlib.Path(a.config).resolve()
if not (str(f).startswith('/etc/nginx/sites-available/') or str(f).startswith('/etc/nginx/conf.d/')):
 sys.exit('只支持 /etc/nginx/sites-available 或 /etc/nginx/conf.d 中的宿主机站点文件')
if not re.fullmatch(r'[a-z][a-z0-9-]{0,47}',a.site_id) or not re.fullmatch(r'[a-z0-9.-]+',a.domain):sys.exit('站点或域名无效')
s=f.read_text();clean=list(s);quote=None;comment=False;escaped=False
for i,c in enumerate(s):
 if comment:
  if c=='\n':comment=False
  else:clean[i]=' '
 elif quote:
  if escaped:escaped=False
  elif c=='\\':escaped=True
  elif c==quote:quote=None
 elif c=='#':comment=True;clean[i]=' '
 elif c in "\"'":quote=c
clean=''.join(clean)
servers=[]
for match in re.finditer(r'\bserver\s*\{',clean):
 start=match.end();depth=1;i=start
 while i<len(clean) and depth:
  depth += (clean[i]=='{')-(clean[i]=='}');i+=1
 if depth:sys.exit('Nginx 花括号不完整')
 body=clean[start:i-1]
 names=re.search(r'\bserver_name\s+([^;]+);',body)
 if names and a.domain in names.group(1).split() and re.search(r'\blisten\s+[^;]*\b443\b[^;]*\bssl\b',body):servers.append((start,i-1,body))
if len(servers)!=1:sys.exit('无法唯一识别域名的 HTTPS server 块。请按 docs/DEPLOYMENT.md 手工接入一次 include；不自动改写复杂或面板配置')
start,end,body=servers[0]
# Strip nested blocks so only server-level rewrites/returns are rejected.
top=[];depth=0
for c in body:
 if c=='{':depth+=1
 elif c=='}':depth-=1
 top.append(c if depth==0 else ' ')
if re.search(r'\b(rewrite|return)\b',''.join(top)):sys.exit('server 级 rewrite/return 会先于资源分流，停止自动接入')
inc=f'include /etc/yandu/nginx/current/{a.site_id}.locations.conf;'
if inc in body:print('受管 include 已存在，无需修改');sys.exit(0)
if '/etc/yandu/' in body:sys.exit('检测到其他 Yandu include，停止以避免冲突')
new=s[:end]+'\n    # Yandu managed include\n    '+inc+'\n'+s[end:]
print(''.join(difflib.unified_diff(s.splitlines(True),new.splitlines(True),fromfile=str(f),tofile=str(f)+' (Yandu)')))
if a.dry_run:sys.exit(0)
backup=pathlib.Path('/etc/yandu/backups')/(f.name+'.'+str(time.time_ns()));backup.parent.mkdir(parents=True,exist_ok=True);shutil.copy2(f,backup)
fd,temp=tempfile.mkstemp(dir=f.parent);os.close(fd);pathlib.Path(temp).write_text(new);shutil.copymode(f,temp);os.replace(temp,f)
try:
 subprocess.run(['/usr/sbin/nginx','-t'],check=True)
 subprocess.run(['systemctl','reload','nginx'],check=True)
except subprocess.CalledProcessError:
 shutil.copy2(backup,f);subprocess.run(['/usr/sbin/nginx','-t']);sys.exit('Nginx 校验/重载失败，已还原原站点文件')
record={'file':str(f),'backup':str(backup),'include':inc}
import json
pathlib.Path('/etc/yandu/nginx-integration.json').write_text(json.dumps(record))
