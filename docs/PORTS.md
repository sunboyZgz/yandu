# 端口与监听配置

下列端口均为 TCP。`127.0.0.1` 指当前运行该服务的机器：本机设备、云服务器和业务容器分别有自己的回环地址。

## 本机开发与安装

macOS、Windows 的源码入口和安装服务使用同一组地址：

| 默认监听地址 | 服务 | 用途 | 配置支持 |
| --- | --- | --- | --- |
| `127.0.0.1:18765` | Yandu Agent | 管理页与管理 API | 一键入口固定 |
| `127.0.0.1:18766` | Caddy | 兼容只读映射的文件入口 | 一键入口固定 |
| `127.0.0.1:18767` | Go 鉴权网关 | Caddy 的逐请求鉴权 | 一键入口固定 |
| `127.0.0.1:18768` | frpc 管理 API | 代理状态与重载 | 一键入口固定；配对后使用 |
| `127.0.0.1:18769` | Caddy 管理 API | 配置重载 | 一键入口固定 |
| `127.0.0.1:18770` | 存储 API | 业务读写、Tus 和隧道源站 | 一键入口固定 |
| `127.0.0.1:18771` | SFTPGo REST API | 内部正式文件操作 | 存储一键入口固定 |

服务按所启用的功能监听。共享源码启动器预检查 `18765–18770`，存储入口再检查 `18771`；只读模式不启动 SFTPGo。SFTPGo 的 SFTP、FTP、WebDAV 和独立 Web 管理页在受管配置中关闭。

当前没有 Agent `--port`、`--listen` 或 `YANDU_PORT` 配置项。`--state-dir`、`--bin-dir` 与 `tools.json` 分别管理状态、程序和工具路径，均不改变监听端口。源码和安装实例不能同时占用这组端口。

需要非默认本机端口时，必须修改源码并重新构建匹配的客户端与启动脚本：

| 配置来源 | 需要同步的内容 |
| --- | --- |
| `internal/model/model.go` | 管理、资源、鉴权、Caddy 管理、存储与 SFTPGo 地址常量 |
| `internal/compiler/compiler.go` | frpc 的本机源站端口及 `webServer.port` |
| `internal/reconcile/engine.go`、`storage_tunnel.go` | frpc 状态查询与端口占用检查 |
| `scripts/dev.py`、`scripts/storage-dev.py` | 预检查列表、SFTPGo 绑定地址与后端 URL |
| `internal/storage/bootstrap_runtime.go` | 安装初始化的 SFTPGo 绑定与后端 URL |
| 安装脚本及 `web/vite.config.ts` | 就绪探针与前端开发代理地址 |

Yandu 会重新生成 Caddy、frpc 和开发后端配置，单独修改这些生成文件不能持久改变整套端口。完整源码启动和重建步骤见开发指南。

## 云端 storage-only

| 默认监听地址 | 服务 | 配置位置 / 字段 |
| --- | --- | --- |
| `0.0.0.0:7000` | frps 的 TLS 隧道入口 | `/etc/yandu/frps.toml` 的 `bindAddr` / `bindPort` |
| `127.0.0.1:19090` | frps 存储转发入口 | `proxyBindAddr`、`allowPorts` 和连接包 `storage.remotePort` |
| `127.0.0.1:19091` | Yandu 业务网关 | `/etc/yandu/gateway.json` 的 `listen` |

本机设备主动连接云端 `7000`；业务访问网关 `19091`，网关转发到 `19090`。`19090` 在存储代理注册后监听。storage-only 不需要 Nginx，也不依赖网站的 `80/443` 或旧映射的 SSH 配置通道。

### 修改业务网关端口

例如将云端网关改为 `19092`。用 `sudoedit /etc/yandu/gateway.json` 将 `listen` 改成 `127.0.0.1:19092`，保持原有其他字段。默认回环配置如下：

```json
{
  "listen": "127.0.0.1:19092",
  "upstream": "http://127.0.0.1:19090",
  "allowedCidrs": ["127.0.0.0/8", "::1/128"]
}
```

同步修改两个调用位置：

- 业务服务器的 `YANDU_GATEWAY` 或 SDK `basePath`：`http://127.0.0.1:19092`。
- `/etc/yandu/monitor.json` 的 `gatewayUrl`：同一地址；保留 `deviceId`、`monitorToken` 和文件权限。

重启并检查：

```bash
sudo systemctl restart yandu-gateway
sudo systemctl status yandu-gateway --no-pager
sudo ss -lntp
sudo yandu list --timeout 10s
```

网关配置不热加载。后续 `pair --refresh-monitor` 会重新生成默认 `19091` 的监控地址，需再次更新 `monitor.json`。连接包中记录的 `gatewayUrl` 也由当前配对器写入默认值；它不用于决定本机 frpc 的源站端口。

### 隧道端口的限制

frps 自身支持 `bindPort`，但当前 Yandu 安装器和配对器生成固定的 `serverPort: 7000`，没有 `--server-port` 参数。只改 frps 会导致设备连不上；改变它需要同步修改初始化/安装脚本、配对代码并重新生成和导入连接包。

存储连接包校验明确只允许 `storage.remotePort: 19090`。因此 `19090` 当前不能通过单改 TOML、JSON 或连接包来配置；需要同步修改连接包校验、生成代码和网关上游后再构建。默认部署保持 `7000 → 19090 → 19091`，仅网关业务端口可直接配置。

### 受信内网与 Docker

另一台业务主机接入时，`gateway.json.listen` 使用服务器实际的内网 IP，配置 `tlsCert`、`tlsKey` 和明确的 `allowedCidrs`。网关上游仍必须是云端回环 HTTP 地址。完整 JSON 与证书权限见[云端启动](CLOUD_STARTUP.md#docker-业务接入)。

Ubuntu 的 host 网络业务容器可访问服务器的 `127.0.0.1:19091`。普通 bridge 网络中，容器的 `127.0.0.1` 指容器自身；需显式接入受信内网网关。

## 兼容只读网站映射

| 默认地址 / 端口 | 用途 | 配置方式 |
| --- | --- | --- |
| 网站 `443` | HTTPS 页面与资源 | 已有 Nginx 站点的 `listen`；自动接入要求可识别的 HTTPS server 块 |
| 网站 `80` | standalone 证书申请与 HTTP 入口 | Nginx / Certbot 的网站配置 |
| 云端 SSH `22` | 受限配置同步 | SSH 服务的实际端口；配对使用 `--ssh 主机:端口` |
| 云端 `0.0.0.0:7000` | frps TLS 隧道 | 与上方隧道端口限制相同 |
| 云端 `127.0.0.1:18080` | Nginx → frps 资源转发 | 生成器固定；需同步 `vhostHTTPPort` 和生成的 Nginx 上游才能更改 |
| 示例上游 `127.0.0.1:3000` | 自己部署的业务应用 | 安装器 `--app-upstream http://127.0.0.1:实际端口` |

`3000` 只是 standalone 安装示例，并非 Yandu 自动启动的业务服务。SSH 使用非 `22` 端口时，先完成 SSH 服务配置，再用 `yandu pair ... --ssh 203.0.113.10:2222` 重新配对并导入；更改连接包不会改变 SSH 服务监听。

只读资源的 `18080` 由 `internal/compiler/compiler.go` 生成的 Nginx 配置引用。单改 frps 的 `vhostHTTPPort` 会中断资源转发。

## 文档站开发与静态预览

两种入口默认均监听 `127.0.0.1:5174`，通过进程环境变量 `DOCS_PORT` 配置；端口占用时直接报错，不自动换号。

macOS / Linux：

```bash
DOCS_PORT=5175 npm --prefix docs-site run dev
# 已构建静态产物后，在另一个端口预览：
DOCS_PORT=5176 npm --prefix docs-site run preview
```

Windows PowerShell：

```powershell
$env:DOCS_PORT='5175'
npm --prefix docs-site run dev
# Ctrl+C 停止后取消该终端的自定义值：
Remove-Item Env:DOCS_PORT
```

`DOCS_PORT` 必须为 `1–65535` 的整数，只影响文档服务器。`DOCS_BASE=/yandu/` 控制静态部署路径，不改变端口。生产静态网站的端口由 Nginx 等宿主服务的 `listen` 决定；配置方法见「开发与文档部署」。

## 前端与独立业务示例

| 默认地址 | 用途 | 调整方法 |
| --- | --- | --- |
| Vite 开发端口 `5173` | React 样式 / 前端调试 | `npm --prefix web run dev -- --host 127.0.0.1 --port 5180 --strictPort` |
| `127.0.0.1:3000` | `examples/storage-business` | 当前示例固定监听；修改 `server.mjs` 的 `app.listen` 和两处 Origin 校验，再重启 |

独立 Vite 页面目前不能完成 Agent 的管理会话和 Origin 校验；完整管理功能使用 `18765` 的内嵌页面。示例没有 `PORT` 环境变量，`YANDU_GATEWAY` 只配置其上游：本机开发填 `http://127.0.0.1:18770`，云端默认填 `http://127.0.0.1:19091`。

## 检查端口占用

macOS：

```bash
lsof -nP -iTCP:18765-18771 -sTCP:LISTEN
lsof -nP -iTCP:5174 -sTCP:LISTEN
```

Windows PowerShell：

```powershell
Get-NetTCPConnection -State Listen |
  Where-Object LocalPort -in 18765,18766,18767,18768,18769,18770,18771,5174 |
  Select-Object LocalAddress,LocalPort,OwningProcess
```

Linux 云端使用 `sudo ss -lntp`。占用检查用于确认实际地址和进程；服务与存储健康查询见云端监控指南。
