# Linux 云端服务器启动

本页部署通用应用存储的云端服务（`storage-only`），安装器接受 **Ubuntu 24.04 及更新版本的 x64 系统，包括 26.04**。**先安装云端并取得连接包，再启动本机 Yandu、导入连接包。云端安装时，本机无需启动或在线。** 文件读写需等两端连接后进行。

## 访问地址与端口：先看这里

> **安装前必做：在云安全组和服务器防火墙中放行 TCP 7000，允许本机 Yandu 连接。** 这是默认部署唯一需要新增公网放行的 Yandu 端口；安装器不会自动修改这些规则。已有 SSH 和业务端口按原部署保留。

| 谁来访问 | 访问哪个地址 | 用途与开放范围 |
| --- | --- | --- |
| 本机 Yandu 客户端 | **云服务器公网 IP:7000** | 建立 TLS 隧道；云端监听 `0.0.0.0:7000` |
| 云端同机的业务服务 | **`http://127.0.0.1:19091`** | 存储 API 网关；仅云端本机可访问 |
| 云端网关内部转发 | `127.0.0.1:19090` | 隧道转发入口；仅云端本机可访问，无需业务配置 |
| 浏览器 / App 用户 | 你的业务 HTTPS 地址，通常为 **443** | 由你部署的业务服务提供，Yandu 安装器不创建该入口 |

**7000 是隧道协议端口，不能在浏览器中当作网站打开。19090、19091 默认不开放公网，不能用“云服务器公网 IP:19091”访问。** 上表的 `127.0.0.1` 均指云服务器自身。

本机 Yandu 主动向云端建立连接，因此本机无需公网 IP。云端存储服务仅需服务器 IP，无需域名或 Nginx；浏览器和 App 通过你的业务服务访问文件，当前安装不会自动生成公网资源直链。

## 1. 安装云端服务

**安装前确认：服务器是 Ubuntu 24.04 或更新版本的 x64 系统，带 systemd 和 apt。** 源码安装先检查复用环境；缺少系统依赖时，只有显式加 `--allow-system-install` 才允许补齐。提前检查与系统写入范围见[环境准备](ENVIRONMENT.md)，实际部署验收状态见[验证记录](STORAGE_VERIFICATION.md)。

选择一种安装来源，在对应目录执行下方同一条命令：

| 安装来源 | 执行目录 | 前提与准备过程 |
| --- | --- | --- |
| **源码仓库** | 仓库根目录 | 保持网络可用；检查 Go/Python，编译云端程序并准备 frps；系统依赖安装需显式允许 |
| Linux 云端发布包 | 解压后的包目录 | 已包含 `yandu-cloud` 和 `bin/frps`，无需安装 Go、Python |

```bash
sudo bash ./install-cloud.sh --mode storage-only --server 203.0.113.10 --device owner
```

源码方式也兼容 `sudo bash ./scripts/install-cloud.sh ...`。云端源码安装只准备云端组件，不需要 Node.js、npm 或构建前端；准备失败会停止并显示原因。加上 `--dry-run` 可只预览，不编译、下载或安装。

新服务器需要一次补齐环境并安装时，在上面的安装命令末尾加 `--allow-system-install`。安装会创建 Yandu 系统身份、systemd 服务、`/etc/yandu`、`/usr/local/lib/yandu` 及命令链接；该开关额外允许 apt 和专用 Go 工具目录写入。

将示例 IP 换成真实服务器公网 IP。`--device owner` 为将来接入的本机设备指定 ID，可先使用示例值 `owner`；它不是本机 IP 或主机名，也不要求提前启动本机服务。安装器会把这个 ID 和连接材料写入连接包，稍后由本机导入。

安装快结束、生成连接包时，终端会提示输入至少 12 个字符的口令。输入时不回显字符，按 Enter 确认；交互输入过短时会要求重新输入。本机导入时需输入同一口令。环境检查或服务安装失败会提前退出，不会进入口令提示。

安装完成后：

- `yandu-frps` 和 `yandu-gateway` 已启用并启动，后续开机由 systemd 自动启动。
- 加密连接包保存在 `/etc/yandu/owner.yandu-profile`；自定义设备 ID 时，文件名随之变化。

此时云端可以独立运行并等待本机接入；尚未建立存储隧道，暂时不能读写本机文件。

已有 storage-only 安装时，直接使用下方的启动与检查命令。安装器不会覆盖已有 `/etc/yandu`；旧只读网站映射的扩展见文末维护说明。

### 连接包口令失败后的恢复

如果已看到两个服务启用，随后出现 `WEAK_PASSPHRASE` 或中断了口令输入，说明**服务已安装，配对尚未完成**。重新运行安装器会保护已有配置；此时直接执行独立配对命令，把示例 IP 换成原安装命令中的公网 IP：

```bash
sudo yandu pair --scope storage-only --dir /etc/yandu \
  --device owner --server 203.0.113.10 \
  --out /etc/yandu/owner.yandu-profile
```

重新输入至少 12 个字符的口令，成功后将连接包导入本机即可。弱口令检查在写入连接包和设备归属记录之前完成，因此这种失败可直接重试普通 `pair`，无需重装或使用 `--refresh-monitor`。设备 ID 使用原安装时的值。

`--refresh-monitor` 用于**已经配对成功的同一设备**更新监控凭证，见云端监控指南。若提示连接包文件已存在，应先检查该文件与当前配对状态，或明确选择新的 `--out` 路径；配对器会拒绝覆盖已有文件。

## 2. 将连接包导入本机 Yandu

这里的“交付”就是**把云服务器上的连接包下载到运行 Yandu 的 Windows/macOS 电脑**。本机没有固定接收目录，放到「下载」文件夹即可：

| 位置 | 示例路径 |
| --- | --- |
| 云服务器生成的文件 | `/etc/yandu/owner.yandu-profile` |
| Windows 下载到本机后 | `C:\Users\你的用户名\Downloads\owner.yandu-profile` |
| macOS 下载到本机后 | `~/Downloads/owner.yandu-profile` |

使用你已有的 SSH/SFTP 连接下载该文件。例如在 **Windows 本机 PowerShell** 中执行下面的命令，替换服务器 IP；示例使用可读取该文件的 root SSH 账号：

```powershell
scp root@203.0.113.10:/etc/yandu/owner.yandu-profile "$env:USERPROFILE\Downloads\owner.yandu-profile"
```

如果平时使用其他 SSH 账号、端口或密钥，沿用自己的连接配置；也可以使用已有 SSH/SFTP 工具的下载功能。

下载后：

1. 在 Windows/macOS 上启动本机 Yandu，打开管理页「服务器连接」。
2. 点击「选择加密连接包」，选择刚下载的 `owner.yandu-profile`。
3. 输入**云端安装或配对时设置的连接包口令**，点击「导入并配对」。

无需把文件复制到源码仓库、程序安装目录或 `.yandu-storage-dev`。导入成功后，Yandu 会把连接材料保存到自己的状态目录；后续连接不依赖这个下载文件，可将它移到自己的私有备份位置。

“口令单独传递”只针对两个人分别管理云端和本机的情况：文件通过 SSH/SFTP 下载，口令通过另一渠道告知。如果两端都是你自己操作，记住安装时设置的口令，在导入时输入即可，不需要再发送口令或创建口令文件。

当前默认部署配对一台本机设备，可在该设备上管理多个项目；项目在本机创建并启用。

## 3. 启动与检查

在云服务器执行；首次安装后也可用这组命令核对状态：

```bash
sudo systemctl start yandu-frps yandu-gateway
sudo systemctl is-active yandu-frps yandu-gateway
sudo ss -lntp
```

预期结果：

- 两个服务均为 `active`；云端监听 `0.0.0.0:7000` 和 `127.0.0.1:19091`。
- **本机导入连接包并连上隧道后**，才出现 `127.0.0.1:19090`。此前没有 19090 监听属于正常情况。
- 本机连接后，运行 `sudo yandu list` 应取得在线采样；`sudo yandu disks` 可查看容量。

如果云端服务正常而本机连不上，先核对安全组和防火墙的 **7000/TCP** 规则，以及连接包中的服务器地址。设备离线时，存储请求返回 503 `DEVICE_OFFLINE`，并带有 `Retry-After: 5`。

## 4. 配置业务访问地址

业务服务与 Yandu 网关运行在同一台云服务器时，配置：

```bash
YANDU_GATEWAY=http://127.0.0.1:19091
```

SDK 的 `basePath` 也填这个根地址，不附加 `/storage/v1`。实际存储请求例如 `http://127.0.0.1:19091/storage/v1/projects/demo/capacity`，由业务服务附加项目 Bearer 凭证。项目凭证在本机管理页创建，保存在业务服务端；浏览器使用业务自己的登录会话。

文件请求经过的链路为：

```text
浏览器 / App → 你的业务服务 → 云端 19091 → 云端 19090 → 隧道 → 本机存储
```

网关流式转发上传和下载，不保存整个文件副本。创建项目、配置 SDK 和验证文件读写见文末后续步骤；Docker 或另一台业务主机按下面的对应方式接入。

## Docker 业务接入

Ubuntu 上业务容器使用 `network_mode: host` 时，可直接访问云服务器的回环 19091：

```yaml
services:
  business:
    image: your-business-image
    network_mode: host
    environment:
      YANDU_GATEWAY: http://127.0.0.1:19091
    env_file: ./business-secrets.env
```

普通 bridge 网络中，容器的 `127.0.0.1` 指容器自身，不能使用上面的网关地址，需按下节配置可达的受信内网网关。此示例针对 Ubuntu 云端业务；macOS/Windows Docker Desktop 的网络配置不同。

### 另一台受信内网业务主机

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

证书需覆盖监听主机名称/IP，由业务信任的 CA 签发；密钥仅允许网关身份读取。非回环监听必须配置 TLS 和允许网段，后者按真实连接源地址检查。配置后执行 `sudo systemctl restart yandu-gateway`，将业务的 `YANDU_GATEWAY` / SDK `basePath` 和 `/etc/yandu/monitor.json` 的 `gatewayUrl` 改为 `https://10.20.0.5:19091`。防火墙仅允许对应受信内网来源访问这个端口，19090 继续保持回环。

## 按需查阅：配置与维护

- **修改端口**：见[云端端口配置](PORTS.md#云端-storage-only)。当前 7000 和 19090 受安装器、连接包约束；19091 可通过配置修改。
- **构建与工具依赖**：源码构建云端包使用 `python3 release/build.py --target linux`；安装与启动的工具解析见[依赖检测](DEPENDENCY_TOOLS.md)。
- **服务与监控**：安装器创建独立的 `yandu-frps`、`yandu-gateway` 服务身份，以及 `/etc/yandu/monitor.json` 只读监控配置。日志、监控凭证更新见后续的云端监控指南。
- **已有部署**：可直接在源码根目录执行 `sudo bash ./upgrade-cloud.sh`，自动编译云端程序并默认只更新 CLI，不重启服务，无需先打包或构建前端；同一命令也适用于 Linux 发布包。网关更新使用 `--component gateway --drain-timeout 300`，隧道更新使用 `--component tunnel --allow-disconnect`；先加 `--dry-run` 预览，源码模式不编译或下载，具体重启计划在正式构建后计算。旧网关首次切换需维护窗口，后续支持等待当前传输结束。卸载使用 `uninstall-cloud.sh`，保留 TLS、配对和恢复记录。旧只读网站映射需明确扩展 frps 允许端口、网关和连接包，不能直接用 storage-only 安装覆盖原站点配置。

平台验收状态见[验证记录](STORAGE_VERIFICATION.md)。
