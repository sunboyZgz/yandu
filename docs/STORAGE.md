# 通用应用存储（v0.2）

本机保存原件，业务通过私有 TLS 隧道按项目访问目录、对象与版本。云端网关流式转发上传和下载，不保存整盘副本。

| 项目模式 | 用途 |
| --- | --- |
| `application_storage` | 应用文件读写、版本、回收站与媒体处理 |
| `readonly_mapping` | 兼容已有网站资源映射；旧项目缺省保持此模式 |

目标平台为 macOS 14+ Apple Silicon、Windows 11 x64，云端 Ubuntu 24.04 x64。实测与发布状态见[验证记录](STORAGE_VERIFICATION.md)。

## 组件与端口

| 组件 | 职责 |
| --- | --- |
| SFTPGo | 独立进程管理正式文件与项目隔离 |
| tusd / tus-js-client | 标准 Tus 续传、上传锁与浏览器重试 |
| SQLite | 名称、虚拟目录、对象、版本与任务状态 |
| frp / Go ReverseProxy | TLS 穿透与云端流式网关 |
| Sharp、FFmpeg、libheif、ExifTool | 图片、HEIC、视频及元数据处理 |

组件版本和 SHA256 由 release 锁文件与 npm 锁文件管理，路径解析见[依赖配置](DEPENDENCY_TOOLS.md)。本机默认回环 18765 / 18770 / 18771；云端默认 TLS 7000、回环 19090 / 19091，修改方法见[端口配置](PORTS.md)。

## 请求与文件生命周期

1. 业务创建上传意图，指定目录、名称、长度、处理预设和预期版本。
2. Tus 将字节写入本机暂存；续传逐次鉴权。
3. 校验、SFTPGo 入库与数据库提交成功后，版本成为 `READY`。
4. 下载通过对象/版本 ID 读取，支持 HEAD、Range、ETag 和条件请求。

替换采用版本条件检查，新版本就绪前保留旧版本。软删除立即禁止读取，再执行可恢复移动。原件版本、回收站、媒体产物及活动任务预留均参与容量控制。

媒体默认只保存原件；指定预设后执行，也可随后创建任务。Worker 只写临时工作区，结果经 SFTPGo 入库，提交前重查版本、删除状态与授权。

项目凭证按操作授权并检查归属，保留在业务服务器。浏览器使用业务会话中转。名称与目录层级存入数据库，物理路径使用对象和版本 ID。

## 开发与安装入口

| 环节 | 入口 |
| --- | --- |
| macOS 源码 | `python3 scripts/storage-dev.py` |
| Windows 源码 | `python .\scripts\storage-dev.py` |
| 本机安装 | 完整原生存储包的安装脚本 |
| Ubuntu 云端 | `install-cloud.sh --mode storage-only` |

源码状态默认 `.yandu-storage-dev`；只调试原件加 `--no-media`。已有文件须通过明确导入进入空存储项目，旧只读项目不自动转换。

OpenAPI 在 `schemas/storage.openapi.json`，TypeScript 服务端 SDK 在 `sdk/typescript`，业务中转与导入示例在 `examples/storage-business`。
