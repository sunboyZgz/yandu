#!/bin/bash
set -euo pipefail
[[ $EUID == 0 && -f '/Library/Application Support/Yandu/installation.json' ]] || { echo '需要 sudo 和安装归属记录';exit 1; }
for service in core sftpgo;do launchctl bootout "system/com.yandu.$service" 2>/dev/null || true;rm -f "/Library/LaunchDaemons/com.yandu.$service.plist";done
rm -rf /Library/Yandu
echo '已移除服务和程序。存储、数据库与服务身份保留，便于恢复；不会自动清空回收站。'
