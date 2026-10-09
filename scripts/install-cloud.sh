#!/usr/bin/env bash
set -euo pipefail
YANDU_SOURCE="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
MODE=existing-nginx SITE=main DOMAIN= CONFIG= UPSTREAM= ALLOW= DEVICE=owner SERVER= DRY=0
while (($#)); do
 case "$1" in
  --mode) MODE=$2; shift 2;; --site-id) SITE=$2;shift 2;; --domain) DOMAIN=$2;shift 2;; --server-config) CONFIG=$2;shift 2;; --app-upstream) UPSTREAM=$2;shift 2;; --allow) ALLOW=$2;shift 2;; --device) DEVICE=$2;shift 2;; --server) SERVER=$2;shift 2;; --dry-run) DRY=1;shift;; *) echo "未知参数：$1" >&2;exit 1;;
 esac
done
[[ $EUID == 0 ]] || { echo "请使用 sudo 运行安装器" >&2;exit 1; }
[[ $SITE =~ ^[a-z][a-z0-9-]{0,47}$ && $DEVICE =~ ^[a-z][a-z0-9-]{0,47}$ && $DOMAIN =~ ^[a-z0-9.-]+$ && $DOMAIN == *.* && $ALLOW =~ ^/([A-Za-z0-9_-]+/)+$ ]] || { echo "需要 --domain 和 --allow /blog/；参数格式无效" >&2;exit 1; }
[[ $MODE == existing-nginx || $MODE == standalone ]] || { echo "模式仅支持 existing-nginx 或 standalone" >&2;exit 1; }
[[ $(uname -s) == Linux && $(uname -m) == x86_64 ]] || { echo "首版云端安装器支持 Linux x64" >&2;exit 1; }
. /etc/os-release
[[ $ID == ubuntu && $VERSION_ID == 24.04 ]] || { echo "自动安装基线为 Ubuntu 24.04；其他系统请按部署文档手动接入" >&2;exit 1; }
if [[ $MODE == existing-nginx ]]; then
 [[ -f $CONFIG && -x /usr/sbin/nginx ]] || { echo "需要宿主机 Nginx 和 --server-config" >&2;exit 1; }
 python3 "$YANDU_SOURCE/nginx-integrate.py" --config "$CONFIG" --domain "$DOMAIN" --site-id "$SITE" --dry-run
else
 [[ $UPSTREAM =~ ^http://127\.0\.0\.1:([0-9]{1,5})$ ]] || { echo "standalone 需要明确 --app-upstream http://127.0.0.1:3000" >&2;exit 1; }
 echo "将创建 $DOMAIN 的 Nginx 站点，业务上游为 $UPSTREAM；证书通过 webroot 申请。"
fi
[[ $DRY == 1 ]] && exit 0
[[ -f $YANDU_SOURCE/yandu-cloud && -f $YANDU_SOURCE/bin/frps ]] || { echo "请使用完整 Linux 发布包，缺少 yandu-cloud 或 bin/frps" >&2;exit 1; }
command -v docker >/dev/null && echo "提示：本安装器只修改宿主机 Nginx，不操作容器或面板。"
# Refuse to reuse an installation under a different identity or site.
if [[ -f /etc/yandu/registry.json ]]; then
 python3 - "$SITE" "$DOMAIN" "$ALLOW" <<'PY'
import json,sys
r=json.load(open('/etc/yandu/registry.json'));s=[x for x in r['sites'] if x['id']==sys.argv[1] and x['domain']==sys.argv[2] and sys.argv[3] in x['allowedPrefixes']]
if not s:sys.exit('已有安装参数不同；新增站点或扩大授权需要云端管理员操作，不覆盖注册表')
PY
fi
if [[ ! -f /etc/yandu/registry.json ]];then
 for YANDU_ACCOUNT in yandu-sync yandu-frps;do if id "$YANDU_ACCOUNT" >/dev/null 2>&1;then echo "已有同名账号 $YANDU_ACCOUNT，缺少本工具归属记录，停止安装" >&2;exit 1;fi;done
 for YANDU_OWNED_FILE in /etc/systemd/system/yandu-frps.service /etc/ssh/yandu_authorized_keys /etc/sudoers.d/yandu;do if [[ -e $YANDU_OWNED_FILE ]];then echo "已有同名文件 $YANDU_OWNED_FILE，停止以避免覆盖" >&2;exit 1;fi;done
fi
apt-get update
apt-get install -y openssh-server openssl sudo python3
if [[ $MODE == standalone ]];then apt-get install -y nginx certbot;fi
install -d -m 0700 /etc/yandu /etc/yandu/tls /etc/yandu/backups /etc/yandu/nginx/generations
install -d -m 0755 /usr/local/lib/yandu
if [[ -f /usr/local/lib/yandu/yandu-cloud ]];then cp /usr/local/lib/yandu/yandu-cloud "/etc/yandu/backups/yandu-cloud.$(date +%s)";fi
install -m 0755 "$YANDU_SOURCE/yandu-cloud" /usr/local/lib/yandu/yandu-cloud
install -m 0755 "$YANDU_SOURCE/bin/frps" /usr/local/lib/yandu/frps
if [[ ! -f /etc/yandu/tunnel.token ]];then openssl rand -hex 32 > /etc/yandu/tunnel.token;fi
SERVER=${SERVER:-$DOMAIN}
[[ $SERVER =~ ^[a-z0-9.-]+$ ]] || { echo "服务器地址无效" >&2;exit 1; }
if [[ ! -f /etc/yandu/tls/server.crt ]];then
 openssl req -x509 -newkey rsa:3072 -nodes -keyout /etc/yandu/tls/ca.key -out /etc/yandu/tls/ca.crt -days 3650 -subj "/CN=Yandu Tunnel CA" -addext "basicConstraints=critical,CA:TRUE" -addext "keyUsage=critical,keyCertSign,cRLSign"
 openssl req -new -newkey rsa:3072 -nodes -keyout /etc/yandu/tls/server.key -out /etc/yandu/tls/server.csr -subj "/CN=$DOMAIN"
 cat > /etc/yandu/tls/server.ext <<EXT
subjectAltName=DNS:$DOMAIN
extendedKeyUsage=serverAuth
basicConstraints=CA:FALSE
EXT
 openssl x509 -req -in /etc/yandu/tls/server.csr -CA /etc/yandu/tls/ca.crt -CAkey /etc/yandu/tls/ca.key -CAcreateserial -out /etc/yandu/tls/server.crt -days 365 -extfile /etc/yandu/tls/server.ext
 rm /etc/yandu/tls/server.csr /etc/yandu/tls/server.ext
fi
chmod 0600 /etc/yandu/tls/*.key /etc/yandu/tunnel.token
python3 - "$SITE" "$DOMAIN" "$ALLOW" <<'PY'
import json,pathlib,secrets,sys
base=pathlib.Path('/etc/yandu');r=base/'registry.json'
if not r.exists():
 r.write_text(json.dumps({'sites':[{'id':sys.argv[1],'domain':sys.argv[2],'allowedPrefixes':[sys.argv[3]],'protectedPrefixes':[sys.argv[3]+'api/',sys.argv[3]+'assets/',sys.argv[3]+'posts/',sys.argv[3]+'_next/'],'probeURL':'https://'+sys.argv[2]}],'devices':{}},indent=2));r.chmod(0o600)
if not (base/'state.json').exists():
 g=secrets.token_hex(16);d=base/'nginx/generations'/g;d.mkdir(parents=True,exist_ok=True);(d/(sys.argv[1]+'.locations.conf')).write_text('# Yandu managed generation '+g+'; do not edit\n');(base/'nginx/current').symlink_to('generations/'+g)
 (base/'state.json').write_text(json.dumps({'generation':g,'revisions':{},'sources':{},'routes':{},'operations':{}}));(base/'state.json').chmod(0o600)
 token=(base/'tunnel.token').read_text().strip()
 (base/'frps.toml').write_text('bindAddr = "0.0.0.0"\nbindPort = 7000\nproxyBindAddr = "127.0.0.1"\nvhostHTTPPort = 18080\nauth.method = "token"\nauth.token = '+json.dumps(token)+'\ntransport.tls.force = true\ntransport.tls.certFile = "/etc/yandu/tls/server.crt"\ntransport.tls.keyFile = "/etc/yandu/tls/server.key"\nlog.to = "console"\nlog.level = "warn"\n');(base/'frps.toml').chmod(0o600)
PY
/usr/local/lib/yandu/frps verify -c /etc/yandu/frps.toml
if [[ $MODE == standalone ]];then
 CONFIG="/etc/nginx/sites-available/yandu-$SITE.conf"
 if [[ ! -f $CONFIG ]];then
  install -d -m 0755 /var/www/yandu-acme
  cat > "$CONFIG" <<CONF
server {
 listen 80;
 server_name $DOMAIN;
 location ^~ /.well-known/acme-challenge/ { root /var/www/yandu-acme; }
 location / { return 503; }
}
CONF
  ln -s "$CONFIG" "/etc/nginx/sites-enabled/yandu-$SITE.conf"
  nginx -t;systemctl reload nginx
 fi
 if [[ ! -f /etc/letsencrypt/live/$DOMAIN/fullchain.pem ]];then
  certbot certonly --webroot -w /var/www/yandu-acme -d "$DOMAIN" --register-unsafely-without-email --agree-tos --non-interactive
 fi
 if ! python3 -c 'import pathlib,sys;sys.exit(0 if "listen 443 ssl" in pathlib.Path(sys.argv[1]).read_text() else 1)' "$CONFIG";then
  cat > "$CONFIG" <<CONF
server {
 listen 80;
 server_name $DOMAIN;
 location ^~ /.well-known/acme-challenge/ { root /var/www/yandu-acme; }
 location / { return 301 https://\$host\$request_uri; }
}
server {
 listen 443 ssl;
 server_name $DOMAIN;
 ssl_certificate /etc/letsencrypt/live/$DOMAIN/fullchain.pem;
 ssl_certificate_key /etc/letsencrypt/live/$DOMAIN/privkey.pem;
 location / { proxy_set_header Host \$host; proxy_pass $UPSTREAM; }
}
CONF
  install -d -m 0755 /etc/letsencrypt/renewal-hooks/deploy
  cat > /etc/letsencrypt/renewal-hooks/deploy/yandu-nginx.sh <<'HOOK'
#!/usr/bin/env bash
set -e
/usr/sbin/nginx -t
systemctl reload nginx
HOOK
  chmod 0755 /etc/letsencrypt/renewal-hooks/deploy/yandu-nginx.sh
 fi
fi
python3 "$YANDU_SOURCE/nginx-integrate.py" --config "$CONFIG" --domain "$DOMAIN" --site-id "$SITE"
if ! id yandu-sync >/dev/null 2>&1;then useradd --system --create-home --shell /bin/sh yandu-sync;fi
usermod --lock yandu-sync
if ! id yandu-frps >/dev/null 2>&1;then useradd --system --no-create-home --shell /usr/sbin/nologin yandu-frps;fi
chown root:yandu-frps /etc/yandu /etc/yandu/tls /etc/yandu/frps.toml /etc/yandu/tls/server.key /etc/yandu/tls/server.crt
chmod 0710 /etc/yandu /etc/yandu/tls
chmod 0640 /etc/yandu/frps.toml /etc/yandu/tls/server.key /etc/yandu/tls/server.crt
cat > /etc/systemd/system/yandu-frps.service <<'UNIT'
[Unit]
Description=Yandu frp tunnel server
After=network-online.target
Wants=network-online.target
[Service]
User=yandu-frps
Group=yandu-frps
ExecStart=/usr/local/lib/yandu/frps -c /etc/yandu/frps.toml
Restart=on-failure
RestartSec=5
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=true
PrivateTmp=true
[Install]
WantedBy=multi-user.target
UNIT
systemctl daemon-reload
if python3 -c 'import json,sys;sys.exit(1 if any(s.get("blocked") for s in json.load(open("/etc/yandu/registry.json"))["sites"]) else 0)';then systemctl enable --now yandu-frps;else systemctl disable --now yandu-frps;echo '站点封禁状态保持，未启动资源隧道';fi
if [[ ! -f /etc/yandu/authorized_keys ]];then
 /usr/local/lib/yandu/yandu-cloud pair --device "$DEVICE" --server "$SERVER" --tls-name "$DOMAIN" --site "$SITE" --out "$YANDU_SOURCE/$SITE.yandu-profile"
fi
install -m 0644 /etc/yandu/authorized_keys /etc/ssh/yandu_authorized_keys
cat > /etc/ssh/sshd_config.d/90-yandu.conf <<'SSH'
Match User yandu-sync
    AuthorizedKeysFile /etc/ssh/yandu_authorized_keys
    AuthenticationMethods publickey
    PasswordAuthentication no
    KbdInteractiveAuthentication no
    AllowTcpForwarding no
    AllowAgentForwarding no
    X11Forwarding no
    PermitTTY no
    PermitTunnel no
Match all
SSH
printf 'yandu-sync ALL=(root) NOPASSWD: /usr/local/lib/yandu/yandu-cloud serve --principal %s\n' "$DEVICE" > /etc/sudoers.d/yandu
chmod 0440 /etc/sudoers.d/yandu
visudo -cf /etc/sudoers.d/yandu
/usr/sbin/sshd -t;systemctl reload ssh
printf '安装完成。安全传送 %s.yandu-profile，单独交付口令。\n' "$SITE"
echo '请核对 DNS、HTTPS、SSH 和公网 7000 的云安全组；安装器不关闭防火墙。'
echo '隧道证书有效期 365 天；到期前按 docs/OPERATIONS.md 轮换。'
