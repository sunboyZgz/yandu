---
title: 云端命令行与容量监控
description: 在无桌面的 Linux 云服务器查看本地设备的项目、配额、磁盘、共享容器和离线快照。
---

# 云端命令行与容量监控

云服务器无需桌面环境。新版本 storage-only 安装后，SSH 登录即可运行：

```bash
sudo yandu list
sudo yandu disks
sudo yandu list --json
```

`list` 展示项目和磁盘汇总，`disks` 只展示磁盘表；两者的 `--json` 都返回完整快照。

## yandu 与 yandu-cloud 的关系

云端的两个名称对应同一个程序，支持相同命令。`yandu` 是日常运维入口；`yandu-cloud` 保留实际程序名称，兼容已有安装脚本、systemd 定义和旧网站映射的 SSH 固定命令。

| 云端入口 | 安装后的路径 | 使用场景 |
| --- | --- | --- |
| `yandu` | `/usr/local/bin/yandu`，链接到下列程序 | 日常执行 `list`、`disks`、`pair`、`version` |
| `yandu-cloud` | `/usr/local/lib/yandu/yandu-cloud` | 安装脚本和后台服务通过完整路径调用；也可执行同样的查询命令 |

可用 `readlink /usr/local/bin/yandu` 检查命令链接。

### 实际后台服务

storage-only 模式下，后台职责由 systemd 服务和启动命令确定：

| 服务 | 执行的程序 / 子命令 | 职责 |
| --- | --- | --- |
| `yandu-gateway` | `yandu-cloud storage-gateway --config /etc/yandu/gateway.json` | 流式转发业务存储请求和只读监控查询，默认监听回环 19091 |
| `yandu-frps` | `yandu-cloud tools run frps --config /etc/yandu/tools.json -- -c /etc/yandu/frps.toml`，启动经过验证的官方 frps | 接收本机设备主动建立的 TLS 隧道，入口 7000，存储代理注册到回环 19090 |

`yandu list`、`yandu disks` 是一次性查询，输出结果后退出。网关由 systemd 持续管理；日常查询时保持已有服务运行即可。旧只读映射的 `yandu-cloud serve` 则由 SSH 固定命令调用，处理授权范围内的云端配置同步，完成后退出。

macOS/Windows 包内的 `yandu` 是本机 Agent，使用 `ui`、`status`、`doctor`；云端包的同名命令用于查询、配对和网关运行。

## 默认端口与配置

监控通过云端回环网关 `127.0.0.1:19091` 查询本机，沿用 frps `7000` 和回环转发 `19090`，不新增监听端口。修改网关端口后，更新 `/etc/yandu/monitor.json` 的 `gatewayUrl` 并保留凭证；网关重启方法见[端口配置](./ports.md#修改业务网关端口)。

## 配对与权限

工具路径记录在 `/etc/yandu/tools.json`，查看用 `sudo yandu tools list`；准备规则见[依赖检测](./dependency-tools.md)。

新 storage-only 配对会生成受限的 `/etc/yandu/monitor.json`，并把专用监控凭证放入加密 schema v2 连接包。本机使用新程序、导入该包并连上隧道后，云端即可发现全部受管项目，包括草稿、停用项目和兼容的只读映射。

监控凭证只能访问 `GET /storage/v1/monitor`，不能下载、上传、修改文件或签发项目凭证。项目业务密钥不能枚举全部项目。清单含本机项目目录路径，只交给受信云端运维人员；配置与缓存权限为 `600`，所以示例使用 `sudo`。

受信内网网关使用系统信任的 TLS 证书，并同步修改 `monitor.json.gatewayUrl`。

## 已有部署升级

旧 schema v2 包不含监控凭证，仍可用于原有存储。旧本机程序没有清单接口，必须同时升级云端和本机：

1. 使用新包升级两端程序，保留状态和 TLS 材料。
2. 在云端为原设备更新监控连接包：

```bash
sudo yandu pair --scope storage-only --refresh-monitor \
  --device owner --server 203.0.113.10 \
  --out /etc/yandu/owner-monitor.yandu-profile
```

3. 安全交付新包并在本机「连接」页导入。导入会停用已有项目，核对后重新应用需要启用的项目。
4. 运行 `sudo yandu list` 验证在线采样。

使用真实服务器地址、原设备 ID 和未存在的输出文件名。更新仅允许原 storage-only 设备，不新增设备或改变 tunnel token/CA；本机导入后旧监控凭证失效。更新期间查询可能失败。纯旧网站映射安装没有存储隧道，需先按[云端部署](./cloud-startup.md)明确扩展，不能只靠别名取得远端磁盘清单。

## 项目与磁盘字段

项目表包含 ID、名称、模式、配置状态、存储启用/隔离状态、目录、卷/容器及配额。JSON 字节字段如下：

| 项目字段 | 含义 |
| --- | --- |
| `quotaBytes` | 项目配额上限 |
| `usedBytes` | 全部原件版本、回收站及媒体产物占用 |
| `reservedBytes` | 上传、提交及媒体工作区预留 |
| `remainingQuotaBytes` | `max(0, quotaBytes - usedBytes - reservedBytes)` |
| `volumeId` / `diskId` | 受管卷 / 分配空间的 NTFS 卷或 APFS 容器标识 |

未初始化项目的使用量为未知。只读映射没有存储配额统计，不递归扫描素材盘，显示未知。停用/隔离项目仍显示已初始化的容量；停用不释放占用。文件操作继续使用独立项目业务凭证。

| 磁盘字段 | 含义 |
| --- | --- |
| `totalBytes` | NTFS 卷或 APFS 容器总容量 |
| `freeBytes` | 按本机服务身份可用的空间，区别于项目剩余额度 |
| `reservedBytes` | 同一卷/容器所有项目的活动预留 |
| `availableAfterReservationsBytes` | 空闲减共享预留及 64 MiB 安全余量，最低为 0 |

Windows 按固定 NTFS 卷合并，macOS 按 APFS 容器合并，同一空间的多个项目只计算一次。多个受管 APFS 卷共享容器时，取可用空间较小值，避免忽略卷限额。

合计只覆盖项目实际使用的磁盘/容器，不枚举未受管磁盘。项目配额不能相加作为物理容量。卷不可用、身份改变或项目磁盘未知时，对应磁盘或合计显示未知，JSON 为 `null`。扣除预留后的空间不等于最大上传文件：暂存、提交及媒体工作区仍按实际任务申请预算。

## 在线与离线快照

命令即时查询，默认超时 30 秒，可使用 `--timeout 60s`。成功后将最后快照原子保存到配置旁的 `monitor-last.json`，权限 `600`。网络不可达或 `503 DEVICE_OFFLINE` 时，若有当前配对的历史快照，会明确标注离线、采样时间及快照年龄。

需要在终端持续查看时，可使用 Linux 的 `sudo watch -n 30 yandu list`。`D1`、`D2` 是当前快照的表格标记；JSON 的完整卷/容器 ID 才是长期关联标识。

| JSON / 退出码 | 含义 |
| --- | --- |
| `online: true, stale: false`，0 | 实时采样成功 |
| `online: false, stale: true`，1 | 不可达，显示历史快照 |
| `online: false, stale: false`，1 | 无历史快照或鉴权/协议失败 |

鉴权失败不展示旧缓存；缓存绑定设备、网关和凭证。配置读取失败返回非零并在 stderr 说明。历史容量不能作为当前写入依据。

`online` 仅表示清单采样成功。采集方还需检查 `summary.unavailableDiskCount`、`summary.unassignedProjectCount` 以及项目/磁盘的 `error`。此接口不执行文件读取或媒体转码，不能代替完整链路探针。

## 无桌面的健康检查

```bash
systemctl status yandu-frps yandu-gateway --no-pager
systemctl show yandu-frps yandu-gateway \
  -p Id -p ActiveState -p SubState -p NRestarts
sudo journalctl -u yandu-frps -u yandu-gateway -f
```

两个服务均需 active/running；重启计数持续增加需排查。可每 30～60 秒执行 `sudo yandu list --json`，采集在线状态、时间与容量，按业务阈值告警。自动采集和告警需要自己的监控系统，安装器不会自动发送通知。当前没有云端 `/healthz`、`/readyz`、`/metrics` 接口。

项目级容量接口和真实 Range 读取见[存储 API](../reference/storage-api.md)及[故障排查](./troubleshooting.md)。

## 源码验证

本机须用新代码启动。云端 CLI 可原生构建：

```bash
go build -o .local/yandu-cloud ./cmd/yandu-cloud
.local/yandu-cloud list --config /绝对路径/monitor.json --json
```

配置包含 `deviceId`、`gatewayUrl` 和 `monitorToken`，须与本机导入的连接包一致，不提交仓库、不放进浏览器代码。macOS/Windows 使用同一协议和各自磁盘 API；Linux 本机存储仅用于契约测试。
