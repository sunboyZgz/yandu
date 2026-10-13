# 项目级 MP4 压缩验收记录

2026-10-12，macOS Apple Silicon。本次完成的是通用项目规则，不引入调用方作品字段或全局 10 MB 规则。项目未配置上限时保持旧预设行为；已配置的任务统一使用项目上限，只能按请求明确选择超限策略。

## 实际运行

服务/SDK 为 v0.2.0 开发源码，新增约束版本 1、流水线 `mp4-720-constraints-v1`；每个任务的 `processingVersion` 还包含 Worker、Node、媒体锁文件及已解析 FFmpeg/FFprobe 的指纹。FFmpeg/FFprobe 9.0.2，Node 22.23.1。

原始 Worker 报告为 `.local/mp4-acceptance/report.json`，原生链路报告为 `.local/mp4-acceptance/native-report.json`（含项目、任务 ID、源版本与实际处理指纹），记录契约 SHA256、素材与产物摘要、请求上限、实际字节数、时长和组件版本。报告随重新验收更新，不把示例数值写成业务保证。

| 场景 | 结果 | 产物字节数 | 实测秒数 |
| --- | --- | --- | --- |
| unconstrained | 实测通过 | 1267618 | 12 |
| exact-byte-limit | 实测通过 | 1267618 | 12 |
| one-byte-over-reject | 预期失败通过 | — | — |
| authorized-keep-start | 实测通过 | 757904 | 7.166667 |
| permitted-but-no-trim | 实测通过 | 1267618 | 12 |
| no-valid-prefix | 预期失败通过 | — | — |
| silent-prefix | 实测通过 | 506281 | 5.7 |
| variable-frame-rate-prefix | 实测通过 | 633795 | 7.5 |
| delayed-audio-prefix | 实测通过 | 630860 | 6.309333 |
| real-film-full | 实测通过 | 980921 | 16.083333 |
| real-film-prefix | 实测通过 | 586384 | 8.554667 |
| project-2000000 | 实测通过 | 1267618 | 12 |
| project-10000000 | 实测通过 | 1267618 | 12 |
| project-20000000 | 实测通过 | 1267618 | 12 |
| damaged-input | 预期失败通过 | — | — |

测试下载/读取真实产物，以独立 FFprobe 再探测时长、帧数和音轨，并用 FFmpeg 完整解码；源文件摘要不变。音轨延迟 2 秒的素材裁剪后仍保留相同音画偏移，误差小于 0.1 秒。首次完整压缩 1,267,618 字节，等于上限成功，少一字节的 reject 请求失败；授权裁剪的最终产物 757,904 字节、7.166667 秒。

Sintel 片段来自 [Blender 官方下载](https://download.blender.org/durian/trailer/sintel_trailer-480p.mp4)，© Blender Foundation，CC BY 3.0，测试取开头 16 秒并重封装；原始 URL、摘要及修改记录在报告中。项目来源和授权见 [Blender 项目介绍](https://durian.blender.org/about/)及[Creative Commons 发行说明](https://creativecommons.org/2010/10/27/ton-roosendaal-sintel-producer-and-head-of-blender-institute/)。测试图案、有声/无声及变帧率素材由 FFmpeg 生成。

## 存储与故障契约

真实独立 SFTPGo 使用三个业务项目分别验证 2/10/20 MB 规则和跨项目任务/产物拒绝；同时，SFTPGo + Tus 分段续传 + Node Worker + 生成式公共路由 + Go 云端网关通过：上传响应重试保持 ID、原件 READY 与任务完成分离、等于上限、超限单次失败、授权裁剪、完整原件保留、下载摘要和解码、不同逻辑任务路径互不覆盖。图片/HEIC/封面/HLS 回归通过。

共享契约覆盖三个项目分别设置 2/10/20 MB、任务级覆盖拒绝、规则快照、幂等冲突、单任务查询/取消、跨项目拒绝；运行中的源读取在取消、凭证撤销、项目停用、删除和替换后五秒内停止且不发布派生物。增量迁移保留旧对象/版本/任务/路径及候选清理记录，不伪造旧时长摘要。

候选上传响应丢失/清理失败与事务原子提交有故障注入测试；内部清理不依赖已经撤销的业务凭证。物理磁盘写满、进程强杀全阶段、硬 RSS 隔离仍按平台发布门槛进一步验收。

## 复现

先按开发指南准备原生工具与 npm 依赖。在仓库根目录，用实际解析的路径设置 `YANDU_FFMPEG`、`YANDU_FFPROBE`、`YANDU_HEIF_THUMBNAILER`：

```bash
node scripts/check-media.mjs
node scripts/check-mp4-constraints.mjs
# 可选：加入有授权的实际素材；记录输入摘要，不修改来源。
YANDU_ACCEPTANCE_VIDEO=/absolute/path/video.mp4 node scripts/check-mp4-constraints.mjs
YANDU_TEST_SFTPGO=/absolute/path/sftpgo YANDU_TEST_MEDIA=1 \
  go test -v ./internal/storage -run TestSFTPGoNativeContract -count=1
go test ./...
go vet ./...
go test -race ./internal/storage ./internal/reconcile ./internal/localapi
```

Windows PowerShell：

```powershell
$env:YANDU_FFMPEG='C:\Yandu\bin\ffmpeg.exe'
$env:YANDU_FFPROBE='C:\Yandu\bin\ffprobe.exe'
node scripts/check-mp4-constraints.mjs
$env:YANDU_TEST_SFTPGO='C:\Yandu\bin\sftpgo.exe'
$env:YANDU_TEST_MEDIA='1'
go test -v ./internal/storage -run TestSFTPGoNativeContract -count=1
```

模板、HEIC 解码器按[共享原生验证](STORAGE_VERIFICATION.md)配置。CI 的两个原生 runner 已加入同一套 MP4 验收，但本次没有执行 Windows CI。

## 尚待验收

- 浏览器实际播放：工具拒绝本地 `file:` 验收页。本次探测和完整解码不能代替浏览器播放；需在目标浏览器播放完整及裁剪产物、跳转尾部并核对音画同步。
- Windows x64 原生运行、Windows 11 SCM/ACL/文件占用与生命周期；交叉构建只属于构建检查。
- Ubuntu 24.04 实际部署及公网 frp 链路；本次原生压缩测试经过 Go 网关，未替代目标云主机验收。
- 多阶段强杀、物理磁盘写满及新管理页的真实交互验收。
