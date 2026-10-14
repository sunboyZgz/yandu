# v0.2 验证与发布门槛

日期：2026-10-11（Asia/Shanghai）。当前环境为 macOS 15.6.1 Apple Silicon，Go 1.26.2、Node 22.23.1。此记录区分实际运行、构建检查和待验收事项；**当前尚未达到计划要求的两平台正式发布条件。**

## 已运行的核心和原生组件

SFTPGo Community 2.7.6 使用锁定上游源码原生构建，独立进程和 SQLite。实际测试了默认管理员初始化、短期 JWT 创建内部用户 API key、独立 home、中文/空格路径、标准 Tus、长度/SHA256 校验、正式移动、READY 提交、Range 下载和内部用户路径越权拒绝。

共享契约测试覆盖项目凭证归属、撤销、活动请求取消、目录成环、配额预留、同名冲突、幂等、续传、遗漏完成通知恢复、条件版本替换、移动响应丢失恢复、软删除与恢复、递归恢复不恢复较早回收项。真实阻塞 HTTP PATCH 在撤销后被 socket deadline 中止，五秒限制通过。增加了存储项目自动重试保持启用的回归测试，以及带校验清单的离线备份/恢复测试。

真实 SFTPGo + Node Worker 也完成 HEIC 上传、处理任务、产物入库和读取，删除后派生物立即不可读。

打包后的 Node、Worker 和全部媒体组件在排除 Homebrew 的 PATH 中通过同一套媒体夹具；真实 SFTPGo 契约使用包内程序再次通过。独立业务示例实际验证了业务会话、生成 SDK 的容量查询、Range 中转及跨项目/凭证端点拒绝。浏览器验证了上传、入库、任务和版本列表。

包内 `storage-bootstrap`、SFTPGo 进程包装器、Agent、Caddy 和三个回环端口也在独立临时状态目录完成初始化、就绪及退出测试。开发启动器收到 Ctrl+C 后退出两个受管子进程，端口释放；页内软删除/恢复与 HEIC 派生物下载入口通过浏览器验收。这些测试使用当前桌面账号，未替代正式低权限服务身份验收。

## 项目级 MP4 压缩补充验证

2026-10-12：固定项目大小上限、规则快照、按任务 ID 查询/取消、授权保留开头、实测结果及原子入库已实现。逐字节边界、真实电影/有声/无声/变帧率素材与 SFTPGo/Tus/云端网关验收见[MP4 压缩记录](MEDIA_ACCEPTANCE.md)。Windows、浏览器播放及新管理页交互仍明确标为待验收。

## 云端项目与磁盘清单补充验证

2026-10-11：新增 `yandu list` / `yandu disks`。在 macOS 实际运行独立 SFTPGo、存储 HTTP 服务、流式网关及原生云端 CLI，读取已入库的 3 字节原件占用、APFS 容器总容量和空闲空间，CLI JSON 成功解析。契约覆盖专用凭证与项目密钥互相拒绝、凭证轮换、管理状态不泄露监控凭证、共享容器去重、停用项目预留、卷缺失/身份变化、离线历史快照、鉴权失败拒绝缓存及重定向拒绝。针对新功能的 race 测试通过。

复现时先 `go build -o .local/yandu-cloud ./cmd/yandu-cloud`，再追加 `YANDU_TEST_CLOUD_CLI="$PWD/.local/yandu-cloud"` 到原生 SFTPGo 契约命令。统计不代替文件读取或媒体探针。Linux 云端包重新构建，Windows 客户端完成交叉构建检查；Ubuntu systemd 生命周期及 Windows NTFS 实测仍待完成。

## 依赖检测与私有路径补充验证

2026-10-11：在 macOS 上通过新源码入口启动实际 SFTPGo 与 Agent，并生成 `tools.json` / `build-tools.json`。已有正确版本的 Caddy、Node 和私有组件直接复用，匹配锁文件且可加载的 npm 模块复用；单独将正确 FFmpeg/FFprobe 加入启动进程 PATH 后，准备结果记录 `system` 来源，没有重新安装它们。`--only ffmpeg` 原生准备器仅输出 FFmpeg/FFprobe。

使用注册表中的 Node、Worker 和媒体环境再次通过真实 SFTPGo + HEIC 处理契约。工具单元测试覆盖 PATH 优先、错误版本回退、仅安装缺失项、显式路径失败、配置损坏不覆盖、SHA256 变化及归档路径穿越/链接；工具、云端和 CLI race 测试通过。Python 启动器 13 项测试通过；Windows 客户端及工具测试交叉编译通过。

storage-only 初始化由 Go 标准库生成 TLS 证书、私有密钥和配置，IP/DNS SAN、证书链、权限及重复初始化拒绝已测试。实际 Ubuntu systemd 与 Windows 服务身份验证仍按下方门槛补齐。

## 媒体原生测试

Sharp 0.35.5、ExifTool vendored 13.59.3、FFmpeg/FFprobe 9.0.2、libheif 1.23.6、libde265 1.0.16。原生构建使用 macOS 14 部署目标，依赖库从锁定源码构建并封装，不依赖运行机 Homebrew。正式打包检查固定版本、原始 SFTPGo 模板、原生库加载路径、SHA256 和许可材料。

项目自行生成的夹具验证：HEIC 多图中的主图选择、HEIC irot 方向 6、JPEG EXIF 方向 6、sRGB 颜色及不放大、小图缩略图、视频封面、720p H.264 MP4 和 HLS ENDLIST/分片。测试代码及图片在 `scripts/check-media.mjs`、`tests/fixtures`，来源按项目 MIT 许可保留。

## 1 GiB 流式测量

macOS 上实际启动独立 Go 网关和官方 frps/frpc 0.71.0，以证书验证的 TLS 隧道分别上传、下载 1,073,741,824 字节，SHA256 一致。每次只有一个传输。

| 记录 | 实际结果 |
| --- | --- |
| 云端暂存增长 | 0 字节 |
| 网关 RSS 峰值增量 | 344,064 字节（约 0.33 MiB） |
| frps RSS 峰值增量 | 24,330,240 字节（约 23.2 MiB） |
| 本机两个传输及校验用时 | 2.907 秒 |

测试上游是持续产生/读取字节的 HTTP 服务，用来验证隧道和网关的流式机制；它没有把 1 GiB 写入 SFTPGo，因此**不代表完整磁盘上传、公网吞吐或 Windows 结果**。另有 Go 网关 1 GiB 契约测试记录堆内存，不能用堆内存替代这里的进程 RSS。

随后在独立测试项目实际完成 **标准 Tus → SFTPGo 校验与移动 → READY 提交 → frp TLS → 网关下载**。上传及下载各 1 GiB，SHA256 相同，逐个传输；两次传输和入库用时 9.248 秒。

| 真实存储链路记录 | 实际结果 |
| --- | --- |
| 云端暂存增长 | 0 字节 |
| 网关 RSS 峰值增量 | 278,528 字节（约 0.27 MiB） |
| frps RSS 峰值增量 | 38,404,096 字节（约 36.63 MiB） |
| 本机 frpc RSS 峰值增量 | 38,584,320 字节（约 36.80 MiB） |
| 本机核心 RSS 峰值增量 | 5,472,256 字节（约 5.22 MiB） |
| SFTPGo RSS 峰值增量 | 11,517,952 字节（约 10.98 MiB） |

此实测发生在 macOS 的本机进程、APFS 与回环网络上，TLS 证书校验开启；没有替代 Windows、Ubuntu 部署或公网/休眠唤醒验收。原始记录为 `.local/storage-native-stream-report.json`。

## 复现

先按开发指南准备原生工具和 npm 依赖：

```bash
go test ./...
go vet ./...
go test -race ./internal/toolchain ./internal/storage ./internal/cloud ./internal/reconcile
python3 scripts/test_dev.py
YANDU_TEST_SFTPGO="$PWD/.local/storage-bin/sftpgo" \
YANDU_TEST_MEDIA=1 \
YANDU_HEIF_THUMBNAILER="$PWD/.local/heif-sealed/bin/heif-thumbnailer" \
YANDU_FFMPEG="$PWD/.local/ffmpeg-9.0.2/bin/ffmpeg" \
YANDU_FFPROBE="$PWD/.local/ffmpeg-9.0.2/bin/ffprobe" \
go test -v ./internal/storage -run TestSFTPGoNativeContract -count=1
YANDU_HEIF_THUMBNAILER="$PWD/.local/heif-sealed/bin/heif-thumbnailer" \
YANDU_FFMPEG="$PWD/.local/ffmpeg-9.0.2/bin/ffmpeg" \
YANDU_FFPROBE="$PWD/.local/ffmpeg-9.0.2/bin/ffprobe" node scripts/check-media.mjs
```

`check-storage-stream.py --gateway <yandu-cloud> --frps <frps> --frpc <frpc>` 复现独立进程流式测量；需要 OpenSSL。默认输出 `.local/storage-stream-report.json`。工具参数使用目标平台原生二进制。

若要测真实存储服务，先准备独立的测试项目、至少 3 GiB 可用空间和测试凭证文件，再追加 `--storage-port 18770 --project-id <测试项目> --key-file <私有凭证文件>`。可用 `--device-pid` 与 `--backend-pid` 记录本机进程 RSS。此模式实际保留 1 GiB 测试原件，测试后应在测试环境执行维护清理，不能指向已有生产项目。

## 必须补齐的发布验收

2026-10-14 云端源码安装入口：仓库根目录和原 `scripts/` 入口均可触发云端组件准备。隔离的安装预检查与准备流程测试覆盖参数转交、dry-run、缺少 Go/Python、已有安装拒绝、组件 SHA256 变化及暂存清理；没有执行真实 Ubuntu systemd 安装。

2026-10-14 运行时版本检查：Go 的模块最低版本保持 1.26.2，Node 改为接受 22.12.0 及以上稳定版。实际使用官方 Node 24.21.0 验证 PATH 复用、实际版本写入工具配置、Vite 生产构建和 Sharp/ExifTool 模块加载；没有下载替代的 Node 22。单元测试覆盖最低版本、较新主版本、过旧版本、预发布版本及持久化；工具链 race、Go 全量测试与 vet 通过。这些检查不替代各运行时版本下的完整媒体任务和系统安装验收。

| 项目 | 当前状态 |
| --- | --- |
| macOS 原生 SFTPGo、存储、媒体 | 已实测 |
| macOS 完整原生组件包 | 已构建；安装身份隔离/生命周期待验收 |
| Windows x64 Go 客户端 | 交叉构建；不表示运行通过 |
| Windows 原生组件与共享契约 | 已提供准备器及 CI，未在本次环境运行 |
| Ubuntu 24.04 及更新版本 storage-only 安装/升级/卸载 | 安装器接受 26.04；模拟系统信息的安装预检查与本机网关检查，实际 Ubuntu systemd 部署待验收 |
| 重启、退出登录、休眠唤醒、服务升级 | 需要两套目标系统 |
| macOS APFS 大小写/容器空间及独立账号权限 | 当前 APFS 标识/预留有功能测试；隔离身份和多卷故障注入待验收 |
| Windows NTFS ACL、文件占用、卷移除、reparse | 适配/脚本及构建检查；Windows 实测待验收 |
| 磁盘实际写满、进程强杀、多阶段响应丢失 | 部分契约恢复测试；完整故障注入矩阵待验收 |
| Worker 更细的 OS 资源隔离 | 已限制任务数、时长、线程、Node 堆及输出预算；独立 Worker 沙箱/硬 RSS 上限未实现 |
| 签名、公证与干净系统无开发依赖 | 原生库已封装；正式发行签名及干净系统验证待完成 |

CI 定义在 `.github/workflows/storage-native.yml`，使用 macOS ARM64 和 Windows 原生 runner 运行同一套契约和媒体测试、打包。GitHub hosted Windows Server 不能替代 Windows 11 桌面、SCM/ACL/重启验收；工作流已写入源码，本次未推送或执行。平台选择参考 [GitHub 官方 runner 文档](https://docs.github.com/en/actions/how-tos/write-workflows/choose-where-workflows-run/choose-the-runner-for-a-job)。
