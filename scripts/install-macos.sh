#!/bin/bash
set -euo pipefail
[[ $EUID == 0 && $(uname -s) == Darwin && $(uname -m) == arm64 ]] || { echo '需要 macOS Apple Silicon 和 sudo';exit 1; }
YANDU_SOURCE=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
YANDU_INSTALL='/Library/Yandu'
YANDU_STATE='/Library/Application Support/Yandu'
YANDU_OPERATOR=${SUDO_USER:-}
[[ -n $YANDU_OPERATOR && $YANDU_OPERATOR != root ]] || { echo '请由需要管理服务的桌面账号通过 sudo 安装';exit 1; }
for file in yandu bin/sftpgo bin/frpc bin/caddy bin/node bin/ffmpeg bin/ffprobe bin/heif-thumbnailer media/worker.mjs component-receipt.json;do [[ -f $YANDU_SOURCE/$file ]] || { echo "完整原生组件包缺少 $file";exit 1; };done
"$YANDU_SOURCE/yandu" storage-components-verify --component-root "$YANDU_SOURCE"
if [[ -e $YANDU_STATE && ! -f $YANDU_STATE/installation.json ]];then echo '目录已存在但没有安装记录，拒绝接管';exit 1;fi
for service in core sftpgo;do launchctl bootout "system/com.yandu.$service" 2>/dev/null || true;done
YANDU_UID=470
for account in _yandu _yandusftpgo;do
 if ! dscl . -read "/Users/$account" >/dev/null 2>&1;then
  while { dscl . -search /Users UniqueID "$YANDU_UID"; dscl . -search /Groups PrimaryGroupID "$YANDU_UID"; } | /usr/bin/grep -q .;do YANDU_UID=$((YANDU_UID+1));done
  dscl . -create "/Groups/$account";dscl . -create "/Groups/$account" PrimaryGroupID "$YANDU_UID"
  dscl . -create "/Users/$account";dscl . -create "/Users/$account" UniqueID "$YANDU_UID";dscl . -create "/Users/$account" PrimaryGroupID "$YANDU_UID";dscl . -create "/Users/$account" UserShell /usr/bin/false;dscl . -create "/Users/$account" NFSHomeDirectory /var/empty;dscl . -create "/Users/$account" IsHidden 1;YANDU_UID=$((YANDU_UID+1))
 elif [[ ! -f $YANDU_STATE/installation.json ]];then echo "同名身份 $account 已存在，拒绝接管";exit 1;fi
done
if ! dscl . -read /Groups/_yanduoperators >/dev/null 2>&1;then dseditgroup -o create _yanduoperators;fi
dseditgroup -o edit -a "$YANDU_OPERATOR" -t user _yanduoperators
install -d -m 0755 "$YANDU_INSTALL"
if [[ -f $YANDU_INSTALL/yandu ]];then
 YANDU_BACKUP="/Library/YanduBackups/$(date +%Y%m%d-%H%M%S)"
 install -d -m 0700 "$YANDU_BACKUP"
 cp -R "$YANDU_INSTALL" "$YANDU_BACKUP/App"
 "$YANDU_INSTALL/yandu" --state-dir "$YANDU_STATE" storage-maintenance --action backup --destination "$YANDU_BACKUP/State"
fi
cp -R "$YANDU_SOURCE/." "$YANDU_INSTALL/"
chown -R root:wheel "$YANDU_INSTALL";chmod -R go-w "$YANDU_INSTALL"
install -d -m 0751 "$YANDU_STATE"
install -d -m 0750 "$YANDU_STATE/Management" "$YANDU_STATE/Storage"
install -d -m 0700 "$YANDU_STATE/storage" "$YANDU_STATE/sftpgo"
"$YANDU_INSTALL/yandu" --state-dir "$YANDU_STATE" storage-bootstrap --component-root "$YANDU_INSTALL"
chown _yandu:wheel "$YANDU_STATE"
chown -R _yandu:_yanduoperators "$YANDU_STATE/Management"
chown _yandu:_yandusftpgo "$YANDU_STATE/Storage"
# Preserve the separate writer identity across upgrades.
for project in "$YANDU_STATE/Storage/"*;do
 [[ -d $project && ! -L $project ]] || continue
 chown _yandu:_yandusftpgo "$project";chmod 0750 "$project"
 chown -R _yandusftpgo:_yandu "$project/files"
 find "$project/files" -type d -exec chmod 0750 {} +
 find "$project/files" -type f -exec chmod 0640 {} +
 chown -R _yandu:_yandu "$project/temporary";chmod -R go-rwx "$project/temporary"
done
chown -R _yandu:_yandu "$YANDU_STATE/storage"
chown -R _yandusftpgo:_yandusftpgo "$YANDU_STATE/sftpgo"
for file in yandu.db yandu.db-wal yandu.db-shm local.token installation.json;do
 [[ ! -f $YANDU_STATE/$file ]] || { chown _yandu:_yandu "$YANDU_STATE/$file";chmod 0600 "$YANDU_STATE/$file"; }
done
for service in core sftpgo;do
 if [[ $service == core ]];then YANDU_ACCOUNT=_yandu;YANDU_ARGS=("$YANDU_INSTALL/yandu" --state-dir "$YANDU_STATE" serve);else YANDU_ACCOUNT=_yandusftpgo;YANDU_ARGS=("$YANDU_INSTALL/yandu" --state-dir "$YANDU_STATE" component-sftpgo);fi
 if [[ $service == core ]];then
 YANDU_XML_ARGS="<string>$YANDU_INSTALL/yandu</string><string>--state-dir</string><string>$YANDU_STATE</string><string>serve</string>"
 else
 YANDU_XML_ARGS="<string>$YANDU_INSTALL/yandu</string><string>--state-dir</string><string>$YANDU_STATE</string><string>component-sftpgo</string>"
 fi
 cat > "/Library/LaunchDaemons/com.yandu.$service.plist" <<PLIST
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
<key>Label</key><string>com.yandu.$service</string>
<key>ProgramArguments</key><array>$YANDU_XML_ARGS</array>
<key>UserName</key><string>$YANDU_ACCOUNT</string>
<key>RunAtLoad</key><true/><key>KeepAlive</key><true/>
<key>ThrottleInterval</key><integer>3</integer><key>Umask</key><integer>23</integer>
<key>EnvironmentVariables</key><dict><key>PATH</key><string>/usr/bin:/bin:/usr/sbin:/sbin:/opt/homebrew/bin:/usr/local/bin</string></dict>
<key>StandardOutPath</key><string>$YANDU_STATE/$service.log</string>
<key>StandardErrorPath</key><string>$YANDU_STATE/$service.log</string>
<key>WorkingDirectory</key><string>$YANDU_INSTALL</string>
</dict></plist>
PLIST
 /usr/bin/plutil -lint "/Library/LaunchDaemons/com.yandu.$service.plist"

 chmod 0644 "/Library/LaunchDaemons/com.yandu.$service.plist"
 touch "$YANDU_STATE/$service.log";chown "$YANDU_ACCOUNT" "$YANDU_STATE/$service.log";chmod 0600 "$YANDU_STATE/$service.log"
 launchctl bootstrap system "/Library/LaunchDaemons/com.yandu.$service.plist"
done
# The operator token is separate from databases and private tunnel credentials.
for i in {1..60};do [[ -f $YANDU_STATE/Management/local.token ]] && break;sleep .5;done
chmod 0640 "$YANDU_STATE/Management/local.token"
chown _yandu:_yanduoperators "$YANDU_STATE/Management/local.token"
# Direct operator ACLs work immediately, even before the login session refreshes group membership.
chmod +a "$YANDU_OPERATOR allow list,search,readattr,readextattr,readsecurity" "$YANDU_STATE/Management"
chmod +a "$YANDU_OPERATOR allow read,readattr,readextattr,readsecurity" "$YANDU_STATE/Management/local.token"
printf '{"schemaVersion":2,"installPath":"/Library/Yandu","statePath":"/Library/Application Support/Yandu"}\n' > "$YANDU_STATE/installation.json"
chmod 0644 "$YANDU_STATE/installation.json"
echo '服务已安装。用 storage-root-macos.sh 创建项目目录；通过 start-macos.command 打开管理页。'
