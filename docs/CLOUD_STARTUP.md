# Linux 云端服务器启动

完整业务链路由云端 frps/网关和本机 Agent/SFTPGo 共同提供；两端运行并配对后，业务才能读写本机文件。目标系统为 Ubuntu 24.04 x64，平台验收状态见[验证记录](STORAGE_VERIFICATION.md)。

## 云端无需网站域名

构建云端包使用 `python3 release/build.py --target linux`。安装后的 `/usr/local/bin/yandu` 链接到云端 `yandu-cloud` 程序；macOS/Windows 的同名程序是本机 Agent。

| 云端默认监听地址 | 用途 | 配置方法 |
| --- | --- | --- |
| `0.0.0.0:7000` | frps TLS 隧道入口 | 当前配对器固定为 7000，单改 `bindPort` 不生效 |
| `127.0.0.1:19090` | 存储隧道转发 | 当前连接包只允许 19090 |
| `127.0.0.1:19091` | 业务与监控网关 | 修改 `/etc/yandu/gateway.json` 的 `listen`，重启网关 |

网关改端口后同步更新业务 `YANDU_GATEWAY` / SDK `basePath` 和 `monitor.json.gatewayUrl`。具体 JSON、重启命令及隧道端口限制见[云端端口配置](PORTS.md#云端-storage-only)。

云端为 Ubuntu 24.04 x64。仅使用服务器 IP 即可部署，无需业务域名或 Nginx：

```bash
sudo bash ./install-cloud.sh --mode storage-only --server 203.0.113.10 --device owner
```

将示例 IP 换成真实服务器 IP。服务器证书的 SAN 支持 IP/DNS，设备校验包内 CA 与服务器名称。安装会提示输入连接包加密口令，包写入 `/etc/yandu/owner.yandu-profile`，通过可信方式交给本机，在「连接」页面导入。连接包 schema v2 包含存储隧道信息，继续支持读取 v1 只读连接包。

安装器创建独立 `yandu-frps`、`yandu-gateway` systemd 服务身份。默认只有 7000/TCP 需要允许本机设备访问；19090、19091 保持回环监听，不直接开放公网。本机没有公网 IP，只需要能够主动建立到云端的 TLS 连接。

安装/启动的工具解析见[依赖检测](DEPENDENCY_TOOLS.md)。配对会生成 `/etc/yandu/monitor.json` 和只读监控凭证；本机导入后可用 `sudo yandu list`、`sudo yandu disks` 查询容量，旧连接包的更新步骤见云端监控指南。

云端业务读取 `http://127.0.0.1:19091/storage/v1/projects/demo/...`，附加项目 Bearer 凭证。设备离线时返回 503 `DEVICE_OFFLINE` 和 `Retry-After: 5`。网关使用 Go ReverseProxy 流式转发，不落盘缓存整个上传或下载。

首版一个隧道端口只配对一个设备、多项目。已有 `/etc/yandu` 安装不会被 storage-only 脚本覆盖。旧只读云端需要明确维护现有 frps 允许端口、网关与连接包，而不能把旧站点配置交给新安装器覆盖。升级/卸载脚本识别两种模式；卸载保留 TLS、配对和恢复记录。


## 已安装服务的启动与检查

首次安装会启用并启动两个服务。后续开机由 systemd 管理，需要手动启动时执行：

```bash
sudo systemctl start yandu-frps yandu-gateway
sudo systemctl is-active yandu-frps yandu-gateway
sudo ss -lntp
```

两个服务应为 `active`，云端监听 7000 和回环 19091。19090 在本机 frpc 注册后才监听；配对前存储请求可能返回 503。

将连接包交给本机，导入后创建并启用存储项目，再从云端执行 `sudo yandu list` 检查在线采样。项目凭证和真实读写的完整验证见快速开始，持续监控见云端监控指南。

## Docker 业务接入

Ubuntu 上建议业务容器使用 `network_mode: host`，仍访问回环 19091，保持网关默认隔离：

```yaml
services:
  business:
    image: your-business-image
    network_mode: host
    environment:
      YANDU_GATEWAY: http://127.0.0.1:19091
    env_file: ./business-secrets.env
```

密钥通过业务服务自己的秘密配置提供，不构建进浏览器镜像。macOS/Windows Docker Desktop 的 host 网络行为不同；此示例针对 Ubuntu 云端业务。

如业务位于另一台受信内网主机，编辑 `/etc/yandu/gateway.json`，明确配置真实监听 IP、允许网段及证书。例如：

```json
{
  "listen": "10.20.0.5:19091",
  "upstream": "http://127.0.0.1:19090",
  "allowedCidrs": ["10.20.0.0/24"],
  "tlsCert": "/etc/yandu/gateway-tls/server.crt",
  "tlsKey": "/etc/yandu/gateway-tls/server.key"
}
```

证书需覆盖监听主机名称/IP，由业务信任的 CA 签发；密钥仅允许网关身份读取。非回环监听缺少 TLS 或允许网段时拒绝启动。允许网段按真实连接源地址检查，不信任浏览器或代理提供的 X-Forwarded-For。配置后重启 `yandu-gateway`，并按相同地址设置业务 SDK。不要将存储网关绑定公网地址并放行全部网段。
