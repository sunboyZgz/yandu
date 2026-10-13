---
title: 常见问题
description: v0.2 的非公网 IP、双平台、开发、云端网关、续传、媒体、配额和旧只读映射问题。
---

# 常见问题

## 没有公网 IP 能用吗？

能。本机主动连接云端 frps 的 TLS 隧道，云端网关访问项目 API，设备不需要公网 IP 或入站映射。服务器需可被设备连出访问；实际公网、休眠唤醒等验收见[记录](/reference/storage-verification)。

## 一定要有域名和 Nginx 吗？

通用存储不需要。可以先完成本机项目和业务示例；云端 storage-only 可用服务器 IP、正确的 TLS SAN 与私有网关。已有网站的 `readonly_mapping` 才沿用 Nginx 和网站授权流程。

## 文件会变成云端整盘副本吗？

不会。云端流式转发，原件、版本、暂存和处理产物在本机；云端不是备份或离线副本。本机关机/断网时业务需要处理 503 `DEVICE_OFFLINE`。

## macOS 和 Windows 都有开发入口吗？

有。macOS Apple Silicon 和 Windows x64 使用 `scripts/storage-dev.py`，状态默认 `.yandu-storage-dev`。Mac 见[开发指南](./development.md)，Windows 见[原生开发](./development-windows.md)。首版 Mac 不覆盖 Intel。

平台实测与发布状态见[验证记录](/reference/storage-verification)。

## `make dev` 或 `dev.ps1` 为什么没有存储后端？

它们是兼容只读映射的入口，只准备传统组件。通用存储使用 `storage-dev.py`，准备 SFTPGo；需要媒体时还准备锁定原生 HEIC/FFmpeg。完整存储安装包也不同于传统 `make release` 组件包。

## 可以修改默认端口吗？

云端业务网关可通过 `gateway.json.listen` 修改；文档站可设置 `DOCS_PORT`。本机 Agent 和存储端口目前固定，`--state-dir` 不改变端口。隧道 7000 和转发 19090 也有配对约束，具体方法见[端口配置](./ports.md)。

## 关闭浏览器会停止服务吗？

后台服务继续运行，但该页面发起的活动上传会中断。开发模式在启动终端 Ctrl+C 停止两个受管进程；安装版由 launchd/SCM 管理。重新打开管理页使用启动器或 `ui`。

## 可以把照片库或非空目录直接接管吗？

通用存储根必须为空且符合平台文件系统要求。新建项目后用[导入示例](./business-integration.md#导入已有文件)明确导入，来源保留。旧只读映射不自动转换。

## 多个应用可以共用一台设备吗？

可以。首版单设备、多项目，每个项目有自己的目录、配额和操作凭证；凭证不能跨项目。不是多设备副本或高可用集群。

## 上传 100% 为什么还不能下载？

Tus 只完成暂存字节，服务还要校验、入库及提交。查询到 `READY` 才能读；管理页显示「正在校验入库」后才显示完成。替换新版本就绪前继续提供旧版本。

## 删除后配额为什么没变？

软删除进入回收站，原件版本和派生物仍计入配额。回收站不自动清空，当前没有永久清空或版本清理入口。暂存/处理还有单独空间预留，不能只按原件大小估算。

## 上传 HEIC 会自动转码吗？

不会。默认只存原件；指定图片预设后解码 HEIC 主图并生成 WebP。也可稍后提交任务。开发 `--no-media` 不提供 HEIC/视频处理，完整流程见[媒体指南](./media.md)。

## 浏览器可以直接拿项目密钥吗？

不应这样接入。密钥只放业务服务端，浏览器用已有业务会话，中转 Tus、原件和 HLS 分片。服务端 SDK 尚未发布 npm，从本地包接入，见[业务示例](./business-integration.md)。

## 可以使用 UNC、网络盘或链接目录吗？

正式存储要求 macOS 受管 APFS 或 Windows 固定 NTFS 卷，不承诺 UNC/网络盘。路径链接、junction/reparse 和多硬链接异常会被拒绝或隔离，按[维护指南](./storage-maintenance.md)排查。

## 安装版要装开发工具吗？

完整原生包带运行程序与媒体组件；编译器、Go、npm、Python 用于开发构建。Mac 包封装原生库，运行时无需 Homebrew。当前下载区中未构建的平台会显示缺失，不能用旧只读包代替完整存储包。

## 旧网站 URL、密码和素材还可以用吗？

可以保留 `readonly_mapping`。旧模式的独立资源 Basic 密码不等于通用存储的项目凭证，站点前缀也不等于对象 ID。按[只读映射指南](./readonly-getting-started.md)操作；其媒体策略、路由释放和部署契约保持该模式的说明。

## 已经可以视为正式双平台发布吗？

当前为 v0.2 开发与验证版，尚未完成双平台正式发布验收。已运行与待完成事项见[验证记录](/reference/storage-verification)。
